[![Go Report Card](https://goreportcard.com/badge/github.com/layer-devops/wrap.sh)](https://goreportcard.com/report/github.com/layer-devops/wrap.sh)
[![Questions or comments?](https://img.shields.io/badge/email-hello%40wrap.sh-blue.svg)](mailto:hello@wrap.sh)

# wrap.sh

A powerful toolkit for running your CI tests.

https://wrap.sh


## Setting up for development

1. Install go
2. Install the Sanic build tool from https://sanic.io
3. Clone this repository and run the following in it:
```
sanic env dev 
sanic run setup_for_development
```

To build the application, run the following in the repository folder:
```
sanic run build
```

To run the tests:
```
sanic run gotest
```

