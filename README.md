[![Go Report Card](https://goreportcard.com/badge/github.com/layer-devops/wrap.sh)](https://goreportcard.com/report/github.com/layer-devops/wrap.sh)
[![Questions or comments?](https://img.shields.io/badge/email-hello%40wrap.sh-blue.svg)](mailto:hello@wrap.sh)

# wrap.sh

A powerful toolkit for running your CI tests.

Allows you to:
- Easily view and edit files in your CI pipelines
- Re-run tests remotely using a web terminal
- Browse websites running locally in the pipeline runner

https://wrap.sh

## How does it work?
The wrap.sh client runs a provided testing command, reporting any output.

If the command fails, the client outputs a link to an online debugging suite for the pipeline.

### Watch it in action
![Preview of wrap.sh](https://wrap.sh/static/wrapsh.gif "Preview of wrap.sh")

## Setup and usage

Enabling wrap.sh in your repository is fast and simple. Just wrap your testing command in one of two ways:

### 1. Through the installer script
Replace "npm run tests" with your testing command in the following:

```
bash <(curl -Ls https://get.wrap.sh) "npm run tests"
```

The script (found in this repository as *wrap.sh*) downloads the latest version of the wrap.sh client and uses it to run your testing command.

### 2. Standalone executable

If you prefer, you can install the client seperately:
```
go get github.com/layer-devops/wrap.sh/src/wrap
```

Then the command to start your tests would resemble the following:
```
wrap "npm run tests"
```

See the quick-start guide for more details: https://wrap.sh/quickstart

## Contributing
Issues, PRs and comments are welcome!

To set up for wrap.sh development:

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

