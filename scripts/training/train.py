#!/usr/bin/env python3
"""Train a local LoRA from explicitly reviewed prompt/answer examples.

Optional ML dependencies are imported only for training, so --validate works
without PyTorch or a GPU. Nothing is uploaded or installed by this script.
"""
import argparse
import json
from pathlib import Path


def load_examples(path):
    examples = []
    for number, line in enumerate(Path(path).read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        row = json.loads(line)
        messages = row.get("messages", [])
        if row.get("reviewed") is not True:
            raise ValueError(f"line {number}: example has not been reviewed")
        if len(messages) != 2 or [m.get("role") for m in messages] != ["user", "assistant"]:
            raise ValueError(f"line {number}: expected one user/assistant pair")
        if any(not isinstance(m.get("content"), str) or not m["content"].strip() for m in messages):
            raise ValueError(f"line {number}: empty or invalid content")
        examples.append(messages)
    if not examples:
        raise ValueError("dataset has no reviewed examples")
    return examples


def encode_example(tokenizer, messages, max_length):
    prompt = tokenizer.apply_chat_template(messages[:1], tokenize=True, add_generation_prompt=True)
    full = tokenizer.apply_chat_template(messages, tokenize=True, add_generation_prompt=False)
    if full[:len(prompt)] != prompt:
        raise ValueError("chat template does not preserve the generation prefix; use a compatible instruct model")
    if len(prompt) >= max_length or len(full) <= len(prompt):
        raise ValueError("example has no trainable answer tokens within max-length")
    ids = full[:max_length]
    return {"input_ids": ids, "attention_mask": [1]*len(ids),
            "labels": [-100]*len(prompt)+ids[len(prompt):]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", required=True)
    parser.add_argument("--model", help="local Hugging Face causal-LM directory (not an Ollama tag)")
    parser.add_argument("--output", default="adapter")
    parser.add_argument("--validate", action="store_true")
    parser.add_argument("--epochs", type=float, default=1)
    parser.add_argument("--max-length", type=int, default=1024)
    parser.add_argument("--rank", type=int, default=8)
    parser.add_argument("--learning-rate", type=float, default=2e-4)
    parser.add_argument("--merge", action="store_true", help="also save merged Safetensors for Ollama import")
    args = parser.parse_args()
    examples = load_examples(args.data)
    if args.validate:
        print(f"Validated {len(examples)} reviewed examples")
        return
    if not args.model:
        parser.error("--model is required for training")
    output = Path(args.output).resolve()
    if output.exists():
        parser.error("--output must be a new directory")
    if args.max_length < 16 or args.rank < 1 or args.epochs <= 0:
        parser.error("invalid training parameters")
    import torch
    from transformers import AutoModelForCausalLM, AutoTokenizer, Trainer, TrainingArguments
    from peft import LoraConfig, get_peft_model

    tokenizer = AutoTokenizer.from_pretrained(args.model, local_files_only=True)
    if not tokenizer.chat_template:
        raise ValueError("model must provide its own chat template")
    if tokenizer.pad_token_id is None:
        tokenizer.pad_token = tokenizer.eos_token
    encoded = [encode_example(tokenizer, m, args.max_length) for m in examples]
    model = AutoModelForCausalLM.from_pretrained(args.model, local_files_only=True)
    model.config.use_cache = False
    model = get_peft_model(model, LoraConfig(r=args.rank, lora_alpha=2*args.rank,
                           lora_dropout=0.05, target_modules="all-linear", task_type="CAUSAL_LM"))

    def collate(rows):
        length = max(len(row["input_ids"]) for row in rows)
        batch = {}
        for key, padding in (("input_ids", tokenizer.pad_token_id), ("attention_mask", 0), ("labels", -100)):
            batch[key] = torch.tensor([row[key]+[padding]*(length-len(row[key])) for row in rows])
        return batch

    trainer = Trainer(model=model, train_dataset=encoded, data_collator=collate,
                      args=TrainingArguments(output_dir=str(output / "checkpoints"),
                         per_device_train_batch_size=1, gradient_accumulation_steps=4,
                         num_train_epochs=args.epochs, learning_rate=args.learning_rate,
                         logging_steps=1, save_strategy="epoch", report_to=[], seed=42))
    result = trainer.train()
    model.save_pretrained(output)
    tokenizer.save_pretrained(output)
    (output / "metrics.json").write_text(json.dumps(result.metrics, indent=2)+"\n", encoding="utf-8")
    if args.merge:
        merged = output / "merged"
        model.merge_and_unload().save_pretrained(merged, safe_serialization=True)
        tokenizer.save_pretrained(merged)
        (merged / "Modelfile").write_text("FROM .\n", encoding="utf-8")
    print(f"Saved trained adapter to {output}; evaluate it before selecting it in offcode.")


if __name__ == "__main__":
    main()
