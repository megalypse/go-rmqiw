APP_NAME ?= rmq
INSTALL_DIR ?= $(HOME)/.local/bin
MOCK_SOURCE ?= rmqiuwpath
RMQIW_PATH ?= $(HOME)/rmqiw
SHELL_PROFILE ?= $(HOME)/.zshrc
UPDATE_SHELL ?= 1

.PHONY: help install-cli init-mocks

help:
	@echo "Targets:"
	@echo "  make install-cli              Build and install the CLI as $(APP_NAME)"
	@echo "  make init-mocks               Create a mock RMQIW_PATH directory"
	@echo ""
	@echo "Variables:"
	@echo "  APP_NAME=$(APP_NAME)"
	@echo "  INSTALL_DIR=$(INSTALL_DIR)"
	@echo "  RMQIW_PATH=$(RMQIW_PATH)"
	@echo "  SHELL_PROFILE=$(SHELL_PROFILE)"
	@echo "  UPDATE_SHELL=$(UPDATE_SHELL)"

install-cli:
	@mkdir -p "$(INSTALL_DIR)"
	@go build -o "$(INSTALL_DIR)/$(APP_NAME)" .
	@if [ "$(UPDATE_SHELL)" = "1" ]; then \
		touch "$(SHELL_PROFILE)"; \
		if grep -F 'export PATH="$(INSTALL_DIR):$$PATH"' "$(SHELL_PROFILE)" >/dev/null 2>&1; then \
			echo "$(INSTALL_DIR) is already configured in $(SHELL_PROFILE)"; \
		else \
			printf '\n# rmqiw CLI\nexport PATH="$(INSTALL_DIR):$$PATH"\n' >> "$(SHELL_PROFILE)"; \
			echo "Added $(INSTALL_DIR) to $(SHELL_PROFILE)"; \
		fi; \
	fi
	@echo "Installed $(APP_NAME) at $(INSTALL_DIR)/$(APP_NAME)"
	@echo 'For the current shell, run: export PATH="$(INSTALL_DIR):$$PATH"'

init-mocks:
	@mkdir -p "$(RMQIW_PATH)/flows"
	@cp "$(MOCK_SOURCE)/config.json" "$(RMQIW_PATH)/config.json"
	@cp "$(MOCK_SOURCE)/flows/"*.json "$(RMQIW_PATH)/flows/"
	@if [ "$(UPDATE_SHELL)" = "1" ]; then \
		touch "$(SHELL_PROFILE)"; \
		if grep -E '^export RMQIW_PATH=' "$(SHELL_PROFILE)" >/dev/null 2>&1; then \
			echo "RMQIW_PATH is already configured in $(SHELL_PROFILE)"; \
		else \
			printf '\n# rmqiw config path\nexport RMQIW_PATH="$(RMQIW_PATH)"\n' >> "$(SHELL_PROFILE)"; \
			echo "Added RMQIW_PATH to $(SHELL_PROFILE)"; \
		fi; \
	fi
	@echo "Created mock config at $(RMQIW_PATH)"
	@echo "For the current shell, run: export RMQIW_PATH=\"$(RMQIW_PATH)\""
