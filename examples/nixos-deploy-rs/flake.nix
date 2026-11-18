{
  description = "NixOS remote deployment simulation sandbox with deploy-rs and Errant";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    deploy-rs = {
      url = "github:serokell/deploy-rs";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      deploy-rs,
    }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        packages = [
          pkgs.deploy-rs
        ];
      };

      nixosConfigurations.nixos-sim = nixpkgs.lib.nixosSystem {
        inherit system;
        modules = [
          (
            { pkgs, ... }:
            {
              networking.hostName = "nixos-sim";

              environment = {
                systemPackages = with pkgs; [
                  vim
                  bash-completion
                  ncurses
                  socat
                ];

                variables = {
                  EDITOR = "vim";
                  VISUAL = "vim";
                };

                extraInit = ''
                  if [ -n "$PS1" ] || [ -t 0 ]; then
                    if [ "$TERM" = "dumb" ] || [ -z "$TERM" ] || ! ${pkgs.ncurses}/bin/infocmp "$TERM" >/dev/null 2>&1; then
                      export TERM=xterm-256color
                    fi
                  fi
                '';

                etc."vimrc.local".text = ''
                  syntax on
                  set background=dark
                  set tabstop=2
                  set shiftwidth=2
                  set expandtab
                  set number
                  autocmd FileType yaml setlocal ts=2 sts=2 sw=2 expandtab
                '';
              };

              boot.loader.grub.device = "/dev/vda";
              fileSystems."/" = {
                device = "/dev/disk/by-label/nixos";
                fsType = "ext4";
                autoResize = true;
              };
              fileSystems."/boot" = {
                device = "/dev/disk/by-label/ESP";
                fsType = "vfat";
              };

              services.openssh = {
                enable = true;
                settings.PermitRootLogin = "prohibit-password";
              };

              systemd.network.enable = true;
              networking.useNetworkd = true;

              programs = {
                bash.completion.enable = true;
                vim = {
                  enable = true;
                  defaultEditor = true;
                };
              };

              # Retain errant sudoer user for SSH access
              users.users.errant = {
                isNormalUser = true;
                extraGroups = [ "wheel" ];
                openssh.authorizedKeys.keys = [
                  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMx6xxUgw9Hxai7xYS/aX60QIFW4xeOBq72lfVSmFOTP xeno@zero"
                ];
              };
              security.sudo.wheelNeedsPassword = false;

              nix.settings.trusted-users = [
                "root"
                "errant"
                "@wheel"
              ];

              system.stateVersion = "26.05";
            }
          )
        ];
      };

      deploy.nodes.nixos-sim = {
        hostname = "192.168.61.10";
        magicRollback = false;
        autoRollback = false;
        profiles.system = {
          user = "root";
          sshUser = "errant";
          magicRollback = false;
          autoRollback = false;
          sshOpts = [
            "-F"
            ".errant/ssh_config"
            "-o"
            "StrictHostKeyChecking=no"
            "-o"
            "UserKnownHostsFile=/dev/null"
          ];
          path = deploy-rs.lib.${system}.activate.nixos self.nixosConfigurations.nixos-sim;
        };
      };

      checks = deploy-rs.lib.${system}.deployChecks self.deploy;
    };
}
