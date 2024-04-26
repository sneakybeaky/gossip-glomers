{
  description = "Aphyrs Maelstrom";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";

  };

  outputs = { self, nixpkgs, flake-utils }:

    let

      # to work with older version of flakes
      lastModifiedDate = self.lastModifiedDate or self.lastModified or "19700101";

      # Generate a user-friendly version number.
      version = builtins.substring 0 8 lastModifiedDate;

      # System types to support.
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];

      # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      # Nixpkgs instantiated for supported system types.
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });

    in
    {

         # Provide some binary packages for selected system types.
      packages = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
            maelstrom = pkgs.stdenvNoCC.mkDerivation rec {

            pname = "maelstrom";
            version = "v0.2.3";
            src = pkgs.fetchzip {
                url = "https://github.com/jepsen-io/maelstrom/releases/download/v0.2.3/maelstrom.tar.bz2";
                hash = "sha256-mE/FIHDLYd1lxAvECZGelZtbo0xkQgMroXro+xb9bMI";
            };

            buildInputs = [ pkgs.jre pkgs.makeWrapper];

            dontBuild = true;
            installPhase = ''
                runHook preInstall

                mkdir -p $out/share
                cp -r . $out/share/maelstrom

                makeWrapper ${pkgs.jre}/bin/java $out/bin/maelstrom \
                    --add-flags "-Djava.awt.headless=true  -jar $out/share/maelstrom/lib/maelstrom.jar"

                runHook postInstall
            '';
            };
        });


      # The default package for 'nix build'. This makes sense if the
      # flake provides only one package or there is a clear "main"
      # package.
      defaultPackage = forAllSystems (system: self.packages.${system}.maelstrom);
    };
}