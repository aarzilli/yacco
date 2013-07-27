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

# building E
cd extra/E
go build
cd -

# building Watch
cd extra/Watch
go build
cd -
