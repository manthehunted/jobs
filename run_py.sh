#!/bin/bash

echo $1
uv run python ./modify_doc.py $1
