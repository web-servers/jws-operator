set -o errexit
set -o nounset

if ! [ -x "$(command -v operator-sdk)" ]; then
  echo 'Install operator-sdk' >&2
  CWD="$(pwd)"
  rm -rf $GOPATH/src/github.com/operator-framework
  mkdir -p $GOPATH/src/github.com/operator-framework
  cd $GOPATH/src/github.com/operator-framework
  git clone https://github.com/operator-framework/operator-sdk
  cd operator-sdk
  git checkout v0.17.x
  export GOPROXY=proxy.golang.org
  make tidy
  make install
  cd $CWD
fi
