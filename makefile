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
# Runs through our suite of all unit tests
#
coverage:
	go test $(TESTING_FLAGS) -cover -coverprofile=coverage.out -timeout $(TIMEOUT) $(PACKAGE)/...

#
# Run the docker container w/ consul so we can run our test suite
#
docker-services:
	docker-compose up