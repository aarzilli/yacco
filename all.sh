#!/bin/bash

set -e

# building yacco
mkdir -p bin
go build
mv yacco bin

# building win
cd extra/win
go build
cd -
mv extra/win/win bin
