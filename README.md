# Ursa&mdash;Preview (don't use yet)

```ascii
                 '
            *          .
                   *       '
              *                *





   *   '*
           *
                *
                       *
               *
                     *
```

Ursa is a collection of _tools and examples_ to work with [Grizzly](http://github.com/xsnout/grizzly), a small data stream processor that aggregates and filters time-stamped data such as `syslog` using a simple custom query language.

The [Grizzly](http://github.com/xsnout/grizzly) repository contains all information about the Ursa Query Language and the project itself.

## Overview

- Examples
- Demos
- API
- Deployment

## Examples

Each example is packaged in a folder as a bundle of 4 files:

- `query.uql` contains the query using the [Ursa query language syntax](http://github.com/xsnout/grizzly/Query.g4)
- `catalog.json` describes the input table schema used by the query and optionally the output schema if it would be reused later to serve as input to a downstream (follow-up) query.
- `spout.cmd` is the command that serves as a code-snippet to feed the Grizzly engine with input (CSV) data.
- `prep.sh` is a Bash script that describes any preparatory activities necessary before we run the engine, for example the generation of a sample CSV input file. If no preparation is necessary, the script is empty except for the Bash preamble.

### Example 1: [Syslog](examples/syslog/)

You can query live syslog data on your computer.

### Example 2: [Finance](examples/finance/)

This example is using live trades provided by the [Finnhub WebSocket Trades API](https://finnhub.io/docs/api/websocket-trades).

You need to have your own `API_KEY`, which you can obtain for free from [Finnhub](https://finnhub.io/).

Go to the file [main.go](cmd/finnhub-trades/main.go) and replace the `API_KEY` value with your own.

Then, run this example as follows:

TODO: add details

### [Synthetic data](examples/synthetic-slice-time-live/) (replay)

Our data generator produces a CSV file that is consistent with the input table schema described in `catalog.json`. The input table name is shown in the `from` clause of the query in the `query.uql` file. This generator is helpful for experimenting with Grizzly and debugging issues.

## Ursa API

The REST API of Ursa allows you to manage all the elements necessary to run a query using a Grizzly engine.

We have a sample client that shows how to use the API. Typically, we will

1. Upload a query file like the ones shown in the examples.
2. Upload a corresponding catalog file that describes the input data.
3. Upload a command file that contains a code snippet that serves as the `stdin` to the engine.
4. Upload a preparation Bash script with optional instructions before the engine is built and run.
5. Create and start a query job.
6. Optionally stop a query job.

You can run several jobs at the same time, they will run as different Grizzly engine processes.

## Ursa API server &ndash; In a terminal

- `make demo-server-start` builds and starts the API server.
- `make demo-server-stop` shuts down the API server.

## Ursa API server &ndash; As a Docker container

You can create a container that runs the API and Grizzly jobs.

Here are the commands that help to manage the container:

- `make deploy` is the _all-in-one_ command to create the `ursa-ursa` image, run the container, and start the API, ready to receive commands from the client.

The `make deploy` command is composed of the following commands that can be used individually.

- `make 1-docker-base-build` creates the base image `ursa-base`. The reason why we have this step is because we need to compile the _Cap'n Proto_ binaries from scratch. It takes a few minutes to finish.
- `make 2-docker-ursa-build` creates the app image `ursa-ursa` based on `ursa-base`. It contains the code to create engines for queries and runs Ursa API.
- `make 3-docker-ursa-up` starts the app container.
- `make 4-docker-ursa-down` stops the app container.
- `make 5-docker-destroy` stops the app container, and removes all images.

## Ursa API client

After the Ursa API server has started, you can issue API calls using a sample client application or cURL commands from the terminal. The latter is more tedious.

### Client application

You can run choose an example by uncommenting the desired `EXAMPLE` definition in the `Makefile`. Then, run

`make demo-client-start`

### Client cURL commands

Example

```sh

```

## Browser view

## Terminal view
