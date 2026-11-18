{
  pkgs ? import <nixpkgs> { },
}:

pkgs.mkShell {
  name = "errant-dev-shell";
  packages = with pkgs; [
    go
    pulumi
    libvirt
    qemu
    cdrkit
    golangci-lint
    gofumpt
    git
    shellcheck
    yamllint
    markdownlint-cli2
    gitleaks
    nixfmt
  ];

  shellHook = ''
    mkdir -p dist
    echo "========================================================"
    echo "  Errant development environment loaded"
    echo "  Run './build.sh' to compile the binary into dist/errant"
    echo "========================================================"
  '';
}
