export REPO_DIR:=$(CURDIR)/repo
export REPOKEY:=D0391BF9
export RETAIN:=1
export DATABASE:=$(shell find $(REPO_DIR) -maxdepth 1 -mindepth 1 -name '*.db.tar.xz' -or -name '*.db.tar.zst')


maintanance: do_updates do_cleanup list_packages upload

do_updates: aur_update
do_updates: repo_update

do_cleanup: cleanup_repo
do_cleanup: cleanup_orphan_signatures
do_cleanup: sign_database

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

aur_update: check_tools check_database
	bash -euo pipefail -c 'for pkg in $$(script_level=1 ./scripts/check_aur_updates.sh); do script_level=1 ./scripts/update-aur.sh $${pkg}; done'

check_aur_update: check_database
	bash ./scripts/check_aur_updates.sh

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
	rm -f *.old*

cleanup_orphan_signatures:
	bash -euo pipefail -c 'for i in $(REPO_DIR)/*.sig; do [[ -f $${i//.sig} ]] || rm $${i}; done'

cleanup_repo: check_tools
	bash ./scripts/do_cleanup.sh

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
