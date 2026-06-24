{ pkgs, lib, config, inputs, ... }:

{
  packages = with pkgs; [
    git
    curl
    gnumake
  ];

  languages.go.enable = true;
}
