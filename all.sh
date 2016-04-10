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

# building y9p
cd extra/y9p
go build
cd -

# building Change
cd extra/Change
go build
cd -

# building LookFile
cd extra/LookFile/
go build
cd -

# building Go
cd extra/Go/
go build
cd -

# building Eqcol
cd extra/Eqcol
go build
cd -
