# Reviewed local learning pipeline

Training is separate from the normal Go runtime and explicitly invoked. Chat
history is not automatically treated as correct training data. Save a prompt
and your corrected/approved answer as UTF-8 files, then add them to a tier's
local dataset:

```sh
bin/offcode learn add -tier forge -prompt-file prompt.txt -answer-file corrected.txt
python3 scripts/training/train.py --data ~/.offcode/training/forge.jsonl --validate
```

This writes one reviewed user/assistant pair per JSONL row. Repeat after
corrections or successful reviewed tasks; the dataset remains on your machine.
There is no implicit upload or background retraining.

In a separate Python environment, install a PyTorch build compatible with the
training hardware, then `pip install -r scripts/training/requirements.txt`.
The reference machine's working Ollama Vulkan backend does not imply that
PyTorch GPU training works on that GPU. CPU training is supported by the script
but full-size tier models may exceed available RAM or take impractically long.

Use a **local Hugging Face instruct-model directory** matching the intended
base model, not an Ollama quantized model tag. The script uses local files only;
it does not download weights or execute remote model code. Train one adapter
per base/tier, in a new output directory:

```sh
python3 scripts/training/train.py \
  --data ~/.offcode/training/forge.jsonl \
  --model /path/to/local/huggingface/base-model \
  --output /path/to/new/forge-adapter --epochs 1 --merge
```

Training uses PEFT LoRA on linear modules, masks user/prompt tokens in the loss,
and saves the adapter, tokenizer and training metrics. The model's own chat
template is required; examples whose prompts leave no answer tokens within
the configured maximum length are rejected. `--merge` additionally exports
merged Safetensors and a `Modelfile` in `output/merged/`. No quality improvement
is assumed: repeat the evaluation campaign and compare against the base model,
including tasks absent from training. Tiny datasets can overfit badly.

For architectures supported by Ollama Safetensors import:

```sh
cd /path/to/new/forge-adapter/merged
ollama create offcode-forge-personal -f Modelfile
ollama run offcode-forge-personal
# Select only after checking behavior and chat-template compatibility:
offcode config set -model offcode-forge-personal
```

Alternatively, import the merged export using offcode itself:

```sh
offcode learn deploy -merged /path/to/new/forge-adapter/merged -model offcode-forge-personal
# After evaluation, -activate also selects it as the default for future sessions.
```

The deploy command requires the training export's `FROM .` Modelfile, invokes
local `ollama create`, and changes saved defaults only with `-activate` after
a successful import. `OLLAMA_HOST`, when set, is carried into the activated
backend URL.

Keep the original base tag and dataset so you can switch back. Do not apply an
adapter trained for one base to another model/tier. Unsupported architectures
need an upstream-supported conversion/deployment path; this script does not
pretend every Ollama tier is trainable/importable on every machine.

For periodic local retraining, run the foreground watcher in the prepared
training environment:

```sh
python3 scripts/training/retrain.py --data ~/.offcode/training/forge.jsonl \
  --model /path/to/local/base --output /path/to/forge-runs --interval 3600
```

It checks the dataset once an hour, trains only changed reviewed data, keeps an
immutable snapshot and log per run, and records success only after training
finishes. It never promotes a model automatically. `--once` checks/trains once;
Ctrl+C stops the watcher. Run one watcher per output directory; after a hard
kill, verify no watcher is running before removing its stale `.retrain.lock`.
No system service or unattended model promotion is installed. A real training run remains
unverified until suitable local weights and the optional ML dependencies are
available.

References: [PEFT LoRA](https://huggingface.co/docs/peft/main/package_reference/lora),
[PEFT configuration](https://github.com/huggingface/peft/blob/main/docs/source/guides/peft_model_config.md),
[Ollama model import](https://docs.ollama.com/import).
