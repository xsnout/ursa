#EXAMPLE              := examples/synthetic-session
#EXAMPLE              := examples/finance
EXAMPLE               := synthetic-slice-time-live
#EXAMPLE              := examples/syslog

JOB                   := /tmp/jobs/$(EXAMPLE)
REPO                  := github.com/xsnout/ursa

all: build demo-server-start

build: clean init demo-build

## ------------------------------------------------------------------
## Start CONSOLE DEMO in one terminal
##
##   make demo-console
##
## ------------------------------------------------------------------
demo-console: demo-console-build demo-console-run

demo-console-build: build copy-repos
	./cmd/scripts/demo-console-build.sh $(EXAMPLE)

demo-console-run:
	./cmd/scripts/demo-console-run.sh $(EXAMPLE)

## ------------------------------------------------------------------
## Start WEB DEMO from local terminals
## ------------------------------------------------------------------

##
## Terminal 1:
##
demo-server-start: demo-stop demo-clean demo-build copy-repos run-api-server
demo-server-stop: demo-stop

##
## Terminal 2:
##
demo-client-start:
#	go run cmd/demo/client/client.go start examples/$(EXAMPLE)
	./cmd/demo/client/client start examples/$(EXAMPLE)

##
## Go to browser using the URL shown in Terminal 2.
##

## ------------------------------------------------------------------
## Run demo in Docker
## ------------------------------------------------------------------

##
## Option 1: All-in-one
##
deploy: 4-docker-ursa-down 5-docker-ursa-destroy 1-docker-base-build 2-docker-ursa-build 3-docker-ursa-up

##
## Option 2: Step-by-step
##

## Build base image
1-docker-base-build: docker-base-build
docker-base-build:
	docker build -t ursa-base:0.1 -f dockerfile-base .

## Build ursa image
2-docker-ursa-build: docker-ursa-build
docker-ursa-build: remove-local-repos 4-docker-ursa-down clean init copy-repos docker-image-ursa-build remove-local-repos

## Run ursa container (Ursa API with Grizzly code)
3-docker-ursa-up: docker-ursa-up
docker-ursa-up:
#	docker compose up --build
	docker compose up
	docker ps -a

## Stop ursa container
4-docker-ursa-down: docker-ursa-down
docker-ursa-down:
	docker compose down

5-docker-ursa-destroy: docker-ursa-destroy
docker-ursa-destroy:
	./cmd/scripts/docker-ursa-destroy.sh
#	docker stop $(docker ps -a -q --filter='name=ursa-ursa')
#	docker rmi $(docker images -a -q "ursa-ursa")

## Stop all containers and delete all images
6-docker-destroy: docker-destroy
docker-destroy: clean
	./cmd/scripts/docker-destroy.sh

## ------------------------------------------------------------------

docker-image-ursa-build:
	docker compose build

docker-ursa-login:
	docker exec -it ursa-ursa sh

###
### The rest is just for debugging:
###

docker-base-run:
	docker container run ursa-base:0.1

docker-base-login:
	./cmd/scripts/docker-image-login.sh ursa-base:0.1

## ------------------------------------------------------------------

demo-client-start-1:
	go run cmd/demo/client/client.go start $(EXAMPLE_1) $(EXAMPLE_1_DATA_SPOUT)

demo-client-start-2:
	go run cmd/demo/client/client.go start $(EXAMPLE_2)

demo-client-stop:
	go run cmd/demo/client/client.go stop all

## ------------------------------------------------------------------

demo-build:
	go build -o cmd/pipe/server/server cmd/pipe/server/server.go
	go build -o cmd/pipe/client/client cmd/pipe/client/client.go
	go build -o cmd/demo/web/server cmd/demo/web/server.go
	go build -o cmd/demo/client/client cmd/demo/client/client.go
	go build -o cmd/finnhub-trades/finnhub-trades cmd/finnhub-trades/main.go
	go build -o cmd/syslog/syslog cmd/syslog/main.go
	go build -o cmd/throttle/throttle cmd/throttle/main.go
	go build -o cmd/datagen/generator cmd/datagen/main.go
	mkdir -p /tmp/demo
	cp -f cmd/pipe/client/client /tmp/demo/demo-pipe-client
	cp -f cmd/pipe/server/server /tmp/demo/demo-pipe-server
	cp -f cmd/demo/web/server /tmp/demo/demo-web-server
	cp -f cmd/demo/client/client /tmp/demo/demo-console-client
	cp -f cmd/finnhub-trades/finnhub-trades /tmp/demo/demo-findata-server
	cp -f cmd/syslog/syslog /tmp/demo/syslog
	cp -f cmd/throttle/throttle /tmp/demo/throttle
	cp -f cmd/datagen/generator /tmp/demo/generator
	cp -rf templates /tmp/demo
	cp -f ./config.yml /tmp/demo

demo-stop:
	if pgrep demo; then pkill demo; fi
	if pgrep grizzly; then pkill grizzly; fi
	if pgrep api-server; then pkill api-server; fi

demo-status:
	ps -ef | grep grizzly
	ps -ef | grep demo
	ps -ef | grep api-server

demo-clean:
	rm -rf repos
	rm -rf tmp
	rm -f cmd/pipe/server/server
	rm -f cmd/pipe/client/client
	rm -f cmd/demo/web/server
	rm -f cmd/demo/client/client

## ------------------------------------------------------------------

run-api-server:
#	go build -o ./ursa github.com/xsnout/ursa/cmd/api-server
	go run github.com/xsnout/ursa/cmd/api-server

run-api-client:
	go run cmd/demo/client/client.go

run-api-curls:
	./cmd/scripts/run-api-curls.sh

copy-repos:
	rm -rf repos
	rm -rf /tmp/repos
	mkdir -p /tmp/repos
#	rsync -avh ../ursa /tmp/repos
	rsync -avh ../grizzly /tmp/repos
	rsync -avh /tmp/repos .

remove-local-repos:
	rm -rf repos

init:
	go mod init $(REPO)
	go mod tidy

clean: demo-clean
	rm -f go.mod
	rm -f go.sum
	rm -f go.work.sum
	rm -f cmd/datagen/generator
	rm -f cmd/demo/web/ws-pipe-webserver
	rm -f cmd/finnhub-trades/finnhub-trades
	rm -f cmd/syslog/syslog
	rm -f cmd/throttle/throttle
	rm -rf /tmp/demo
	rm -rf /tmp/jobs
	rm -rf /tmp/repos
	rm -rf /tmp/uploads
