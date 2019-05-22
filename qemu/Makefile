all: start

prepare: kernel zinit

zinit:
	@echo "copy zinit into overlay"
	cp $(shell which zinit) overlay/sbin/zinit

kernel:
	@echo "Download 0-OS kernel"
	wget https://bootstrap.grid.tf/kernel/zero-os-development-test-v2-generic.efi -O testv2.efi
	ln -sf testv2.efi test.efi

start:
	bash vm.sh -n node1
