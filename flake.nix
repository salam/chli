{
  description = "chli - Unified CLI for Swiss government open data";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = builtins.replaceStrings [ "v" ] [ "" ]
          (builtins.readFile ./VERSION or "dev");
      in {
        packages.default = pkgs.buildGoModule {
          pname = "chli";
          inherit version;
          src = ./.;

          # TODO: Update this hash after first build. Run:
          #   nix build 2>&1 | grep 'got:' | awk '{print $2}'
          vendorHash = pkgs.lib.fakeHash;

          ldflags = [
            "-s" "-w"
            "-X github.com/matthiasak/chli/cmd.version=${version}"
            "-X github.com/matthiasak/chli/cmd.commit=${self.shortRev or "dirty"}"
            "-X github.com/matthiasak/chli/cmd.buildDate=1970-01-01T00:00:00Z"
          ];

          meta = with pkgs.lib; {
            description = "Unified CLI for Swiss government open data";
            homepage = "https://github.com/matthiasak/chli";
            license = licenses.mit;
            mainProgram = "chli";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            goreleaser
          ];
        };
      });
}
