{
  description = "lcp-decrypt";

  inputs = {
    nixpkgs.url      = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url  = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        overlays = [
          (self: prevPkgs: {
            tinygo = prevPkgs.callPackage ./tinygo.nix {};
          })
        ];
        pkgs = import nixpkgs {
          inherit system overlays;
        };
        rev = if (self ? shortRev) then self.shortRev else "dev";
      in
      with pkgs;
      {
        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.biome
            pkgs.go
            pkgs.gopls
            pkgs.tinygo
          ];

	  GOTOOLCHAIN = "local";

          shellHook = ''
          export GOPATH="$(realpath .)/.go";
          export PATH="''\${GOPATH}/bin:''\${PATH}"
          '';
        };

        packages.default = pkgs.buildGo122Module {
          pname = "lcp-decrypt";
          version = rev;
          src = pkgs.lib.cleanSource self;
          vendorHash = "sha256-ouRj0Rav6Og/2lVk6sk/O37xlQJd3fLwPoKlEgb/7ik=";
        };
      }
    );
}
