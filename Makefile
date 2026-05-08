.DEFAULT_GOAL := serve

.PHONY: serve fetch parse clear

# Start the API server
# Usage: STEAM_WEB_API_KEY="your_key" make
serve:
	cd backend && go run ./cmd/cs-insights/main.go --serve

# Parse a specific demo
# Usage: make parse DEMO=./match.dem PLAYER="s1mple"
parse:
	cd backend && go run ./cmd/cs-insights/main.go --demo="$(DEMO)" --player="$(PLAYER)"

# Fetch and process recent matches
# Usage: make fetch STEAM_ID=Miconen COOKIE=123 PLAYER="s1mple" LIMIT=3
fetch:
	cd backend && go run ./cmd/cs-insights/main.go --steam_id="$(STEAM_ID)" --cookie="$(COOKIE)" --player="$(PLAYER)" --limit=$(LIMIT)

# Clear the database
clear:
	cd backend && go run ./cmd/cs-insights/main.go --clear
