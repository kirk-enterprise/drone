root=(../../../..)
source $root/env.sh


make deps
make gen     # Generate code
make build 
#make docker

