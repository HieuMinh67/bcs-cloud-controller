#!/bin/bash

cd packer ||

packer init .
packer fmt .
packer validate .
#packer build -var 'ami_tags={ Name = "foo" }' aws-ubuntu.pkr.hcl
packer build .
#packer build . 2>&1 | sudo tee output.txt
#tail -2 output.txt | head -2 | awk 'match($0, /ami-.*/) { print substr($0, RSTART, RLENGTH) }' > sudo ami.txt

cd ..