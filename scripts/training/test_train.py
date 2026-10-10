import json
import tempfile
import unittest
from pathlib import Path
from train import load_examples, encode_example

class DatasetTests(unittest.TestCase):
    def test_only_reviewed_pairs(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'examples.jsonl'
            row = {'reviewed': True, 'messages': [{'role':'user','content':'Question'}, {'role':'assistant','content':'Answer'}]}
            path.write_text(json.dumps(row), encoding='utf-8')
            self.assertEqual(len(load_examples(path)), 1)
            row['reviewed'] = False
            path.write_text(json.dumps(row), encoding='utf-8')
            with self.assertRaises(ValueError): load_examples(path)

    def test_prompt_is_masked_and_answer_is_trained(self):
        class Tokenizer:
            def apply_chat_template(self, messages, **kwargs):
                return [1, 2] if len(messages) == 1 else [1, 2, 3, 4]
        messages = [{'role':'user'}, {'role':'assistant'}]
        self.assertEqual(encode_example(Tokenizer(), messages, 4)['labels'], [-100,-100,3,4])
        with self.assertRaises(ValueError): encode_example(Tokenizer(), messages, 2)

if __name__ == '__main__': unittest.main()
