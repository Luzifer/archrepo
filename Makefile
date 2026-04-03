BUILD_IMAGE := ghcr.io/luzifer-docker/arch-repo-builder:latest
export REPO_DIR := $(CURDIR)/repo
export REPOKEY := D0391BF9 # Key to sign packages with
export RETAIN := 1 # How many versions of each package to keep
export DATABASE := $(REPO_DIR)/luzifer.db.tar.zst

##@ General

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

maintanance: ## Main entrypoint for day-to-day usage
maintanance: do_updates do_cleanup list_packages upload

rebuild_db: ## Remove database and re-add all still existing packages
rebuild_db: clear_database do_cleanup

do_updates: repo_update

do_cleanup: cleanup_repo
do_cleanup: cleanup_orphan_signatures
do_cleanup: sign_database
do_cleanup: cleanup_files
do_cleanup: list_packages

download: ## Downloads the current repo state
	bash -ec "eval $$(vault2env --key secret/minio/archrepo --export) && s3sync --delete s3://archrepo/x86_64/ $(REPO_DIR)/"

upload: cleanup_files ## Uploads the current repo state
	bash -ec "eval $$(vault2env --key secret/minio/archrepo --export) && s3sync --delete $(REPO_DIR)/ s3://archrepo/x86_64/"

# Maintenance targets

check_database:
	test -n '$(DATABASE)'

check_tools:
	@which column
	@which docker
	@which repo-add
	@which vault2env

cleanup_files:
	rm -f $(REPO_DIR)/*.old*

cleanup_orphan_signatures:
	bash -euo pipefail -c 'for i in $(REPO_DIR)/*.sig; do [[ -f $${i//.sig} ]] || rm $${i}; done'

cleanup_repo: check_tools
	bash ./scripts/do_cleanup.sh

clear_database:
	rm -f $(REPO_DIR)/*.db* $(REPO_DIR)/*.files*

list_packages:
	bash ./scripts/listing.sh >$(REPO_DIR)/packages.txt

repo_update: check_tools load_ssh_key repo-tool/repo-tool
	repo-tool/repo-tool \
		--build-image=$(BUILD_IMAGE) \
		--cache=$(CURDIR)/.repo_cache.yaml \
		--config=$(CURDIR)/repo-urls.yaml \
		--pacman-config=$(CURDIR)/scripts/pacman.conf \
		--remove-unmanaged-packages=true \
		--repo-dir=$(REPO_DIR) \
		--show-already-built=false \
		--signing-vault-key=secret/jenkins/arch-signing

sign_database:
	repo-add -s --key $(REPOKEY) $(DATABASE)

# Helpers

load_ssh_key:
	vault-sshadd loki

repo-tool/repo-tool:
	$(MAKE) -C repo-tool build
