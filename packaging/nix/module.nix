flake:
{ config, lib, pkgs, ... }:

let
  cfg = config.services.claude-cowork;
in
{
  options.services.claude-cowork = {
    enable = lib.mkEnableOption "Claude Cowork Service (native Linux backend)";

    package = lib.mkOption {
      type = lib.types.package;
      default = flake.packages.${pkgs.system}.claude-cowork-service;
      description = "The claude-cowork-service package to use.";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.user.services.claude-cowork = {
      description = "Claude Cowork Service (native Linux backend)";
      after = [ "default.target" ];
      wantedBy = [ "default.target" ];
      serviceConfig = {
        ExecStart = "${cfg.package}/bin/cowork-svc-linux";
        Restart = "on-failure";
        RestartSec = 5;
      };
    };

    environment.systemPackages = [ cfg.package ];
  };
}
