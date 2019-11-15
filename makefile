PACKAGE=github.com/robsignorelli/configify-consul
TIMEOUT=20s
TESTING_FLAGS=
ifeq ($(VERBOSE),true)
	TESTING_FLAGS=-v
endif

#
# Runs through our suite of all unit tests
#
test:
	go test $(TESTING_FLAGS) -timeout $(TIMEOUT) $(PACKAGE)/...

#
# Checks code coverage of our tools. Use the following to visualize gaps:
#
#   go tool cover -html=coverage.out
#
coverage:
	go test $(TESTING_FLAGS) -cover -coverprofile=coverage.out -timeout $(TIMEOUT) $(PACKAGE)/...

#
# Run the docker container w/ consul so we can run our test suite
#
docker-services:
	docker-compose up