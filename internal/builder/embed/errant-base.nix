{ pkgs, ... }:
{
  services.openssh = {
    enable = true;
    settings.PermitRootLogin = "prohibit-password";
  };

  services.cloud-init = {
    enable = true;
    network.enable = true;
    settings = {
      datasource_list = [
        "NoCloud"
        "None"
      ];
      users = [
        "errant"
        "root"
      ];
      default_user = {
        name = "errant";
      };
    };
  };

  networking.firewall.enable = false;
  networking.useNetworkd = true;
  system.stateVersion = "26.05";

  users.users.root.initialHashedPassword = "";
  users.users.errant = {
    isNormalUser = true;
    extraGroups = [ "wheel" ];
    initialHashedPassword = "";
  };
  security.sudo.wheelNeedsPassword = false;

  # Ensure /bin/bash exists for universal compatibility with cloud-init and scripts
  system.activationScripts.binbash = ''
    mkdir -m 0755 -p /bin
    ln -sfn /run/current-system/sw/bin/bash /bin/bash
  '';

  nix.settings.trusted-users = [
    "root"
    "errant"
    "@wheel"
  ];

  environment.systemPackages = [
    pkgs.vim
    pkgs.curl
    pkgs.git
  ];
}
