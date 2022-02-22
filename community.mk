.PHONY: vendor-pre

vendor-pre:
	@sed -i "s#\tgithub.com/arangodb/rebalancer#\t// github.com/arangodb/rebalancer#g" "$(ROOT)/go.mod"
	@sed -i "s#\tgithub.com/arangodb/arangosync#\t// github.com/arangodb/arangosync#g" "$(ROOT)/go.mod"

vendor: vendor-pre