export GOPATH=$(cd ../../../../;pwd)
export PATH="$PATH:$GOPATH/bin"

#make deps
make gen     # Generate code
make build 

docker build -t k-drone  -f Dockerfile.amd64 .

