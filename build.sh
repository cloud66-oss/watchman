#!/bin/bash

docker build -t cloud66/watchman:1.0.0-$(git rev-parse --short HEAD) --build-arg rev=$(git rev-parse --short HEAD) .
docker push cloud66/watchman:1.0.0-$(git rev-parse --short HEAD)