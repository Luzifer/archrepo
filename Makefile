export REPO_DIR:=$(CURDIR)/repo
export REPOKEY:=D0391BF9
export RETAIN:=1
export DATABASE:=$(REPO_DIR)/luzifer.db.tar.zst


maintanance: do_updates do_cleanup list_packages upload

do_updates: repo_update

do_cleanup: cleanup_repo
do_cleanup: cleanup_orphan_signatures
do_cleanup: sign_database
do_cleanup: cleanup_files
do_cleanup: list_packages

download:
	vault2env --key=secret/aws/private -- aws s3 sync \
		--delete \
		--exclude '*.old*' \
		--exclude '.git/*' \
		--acl=public-read \
		s3://arch-luzifer-io/repo/x86_64/ $(REPO_DIR)/

upload: cleanup_files check_archive_mix
	vault2env --key=secret/aws/private -- aws s3 sync \
		--delete \
		--exclude '*.old*' \
		--exclude '.git/*' \
		--acl=public-read \
		$(REPO_DIR)/ s3://arch-luzifer-io/repo/x86_64/

# Maintenance targets

check_database:
	test -n '$(DATABASE)'

check_tools:
	@which aws
	@which column
	@which curl
	@which docker
	@which jq
	@which repo-add
	@which vault
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
	tar -tf $(DATABASE) | grep -v '/desc' | sed -E 's/(.*)-([^-]+-[0-9]+)\//\1\t\2/' | sort | column -t >$(REPO_DIR)/packages.txt

repo_update: check_tools load_ssh_key
	bash -euo pipefail -c 'for repo in $$(grep -v "^#" repo-urls); do script_level=1 ./scripts/update-repo.sh $${repo}; done'

sign_database:
	repo-add -s --key $(REPOKEY) $(DATABASE)

# Helpers

check_archive_mix:
	bash ./scripts/has_archive_mix.sh

load_ssh_key:
	vault-sshadd loki
