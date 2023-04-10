#!/bin/sh

## Install requirements && run load_test. Your fileserver stack must be running.
export FILE_SERVER_ADDR="http://localhost:1234"

## If you do'nt have python installed, follow these directions

python -m ensurepip --upgrade
pip install -r requirements.txt
python main.py