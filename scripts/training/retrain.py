#!/usr/bin/env python3
"""Periodically train new adapters when a reviewed dataset changes.

Runs in the foreground, never promotes a model automatically. Stop with Ctrl+C.
Each run trains an immutable dataset snapshot in its own output directory.
"""
import argparse
import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile
import time
from train import load_examples


def run_once(data, model, output):
    raw = data.read_bytes()
    digest = hashlib.sha256(raw).hexdigest()
    state = output / 'last-success.sha256'
    if state.exists() and state.read_text().strip() == digest:
        print('Reviewed dataset unchanged; no training needed.', flush=True)
        return
    run = Path(tempfile.mkdtemp(prefix='run-', dir=output))
    snapshot = run / 'reviewed.jsonl'
    snapshot.write_bytes(raw)
    load_examples(snapshot)
    command = [sys.executable, str(Path(__file__).with_name('train.py')),
               '--data', str(snapshot), '--model', str(model),
               '--output', str(run / 'adapter'), '--merge']
    with (run / 'training.log').open('w', encoding='utf-8') as log:
        process = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT)
    if process.returncode:
        raise RuntimeError(f'Training failed; inspect {run / "training.log"}')
    temp = output / 'last-success.tmp'
    temp.write_text(digest+'\n', encoding='utf-8')
    temp.replace(state)
    print(f'New adapter ready for evaluation in {run / "adapter"}', flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--data', type=Path, required=True)
    parser.add_argument('--model', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--interval', type=int, default=3600)
    parser.add_argument('--once', action='store_true')
    args = parser.parse_args()
    if args.interval < 60:
        parser.error('--interval must be at least 60 seconds')
    args.output.mkdir(parents=True, exist_ok=True)
    # Exclusive lock prevents two watchers from racing the same output state.
    lock = args.output / '.retrain.lock'
    with lock.open('x') as handle:
        handle.write(str(__import__('os').getpid()))
    try:
        while True:
            try:
                run_once(args.data.resolve(), args.model.resolve(), args.output.resolve())
            except Exception as exc:
                print(str(exc), file=sys.stderr, flush=True)
                if args.once:
                    return 1
            if args.once:
                return 0
            time.sleep(args.interval)
    finally:
        lock.unlink(missing_ok=True)


if __name__ == '__main__':
    raise SystemExit(main())
