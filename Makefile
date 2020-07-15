export DATABASE:=$(shell find . -maxdepth 1 -mindepth 1 -name '*.db.tar.xz' -or -name '*.db.tar.zst')
export REPOKEY:=D0391BF9


maintanance: do_updates do_cleanup upload

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
		s3://arch-luzifer-io/repo/x86_64/ $(CURDIR)/

upload: cleanup_files check_archive_mix
	vault2env --key=secret/aws/private -- aws s3 sync \
		--delete \
		--exclude '*.old*' \
		--exclude '.git/*' \
		--acl=public-read \
		$(CURDIR)/ s3://arch-luzifer-io/repo/x86_64/

# Maintenance targets

aur_update: check_tools check_database
	bash -euo pipefail -c 'for pkg in $$(script_level=1 ./scripts/check_aur_updates.sh); do script_level=1 ./scripts/update-aur.sh $${pkg}; done'

check_aur_update:
	bash ./scripts/check_aur_updates.sh

check_database:
	test -n '$(DATABASE)'

check_tools:
	@which aws
	@which curl
	@which docker
	@which jq
	@which repo-add
	@which repoctl
	@which vault
	@which vault2env

cleanup_files:
	rm -f *.old* scripts/repoctl.toml

cleanup_orphan_signatures: check_database
	bash -euo pipefail -c 'for i in *.sig; do [[ -f $${i//.sig} ]] || rm $${i}; done'

cleanup_repo: check_tools check_database scripts/repoctl.toml
	repoctl update

repo_update: check_tools check_database
	bash -euo pipefail -c 'for repo in $$(grep -v "^#" repo-urls); do script_level=1 ./scripts/update-repo.sh $${repo}; done'

scripts/repoctl.toml:
	./scripts/repoctl.sh

sign_database:
	repo-add -s --key $(REPOKEY) $(DATABASE)

# Helpers

check_archive_mix:
	bash ./scripts/has_archive_mix.sh
