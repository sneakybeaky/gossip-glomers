{
  description = "A Nix-flake-based Go development environment";

  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1"; # unstable Nixpkgs
    maelstrom = {
      url = "path:./maelstrom-flake";
      inputs.nixpkgs.follows = "nixpkgs"; };
  };

  outputs =
    { self, ... }@inputs:

    let
      goVersion = 26; # Change this to update the whole stack

      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forEachSupportedSystem =
        f:
        inputs.nixpkgs.lib.genAttrs supportedSystems (
          system:
          f {
            inherit system;
            pkgs = import inputs.nixpkgs {
              inherit system;
              overlays = [ inputs.self.overlays.default inputs.self.overlays.maelstrom ];
            };
          }
        );
    in
    {
      overlays = {
        default = final: prev: {
          go = final."go_1_${toString goVersion}";
        };

        maelstrom = final: prev: {
          maelstrom = inputs.maelstrom.packages.${final.stdenv.hostPlatform.system}.maelstrom;
        };
      };

      devShells = forEachSupportedSystem (
        { pkgs, system }:
        {
          default = pkgs.mkShellNoCC {
            packages = with pkgs; [
              # go (version is specified by overlay)
              go
              graphviz
              gnuplot
              curl
              glow
              self.formatter.${system}
              go-task
              maelstrom
            ];

            shellHook = ''
              export GOPATH="$PWD/.bin"
              export PATH="$PATH:$PWD/.bin"
            '';


          };
        }
      );

      formatter = forEachSupportedSystem ({ pkgs, ... }: pkgs.nixfmt);
    };
}
