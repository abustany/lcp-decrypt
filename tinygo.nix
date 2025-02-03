{ lib, stdenv, fetchurl, autoPatchelfHook }:

let

  version = "0.34.0";

  sources = {
    "x86_64-darwin" = {
      url = "https://github.com/tinygo-org/tinygo/releases/download/v${version}/tinygo${version}.darwin-amd64.tar.gz";
      sha256 = lib.fakeSha256;
    };
    "aarch64-darwin" = {
      url = "https://github.com/tinygo-org/tinygo/releases/download/v${version}/tinygo${version}.darwin-arm64.tar.gz";
      sha256 = "sha256-apuuTleq+L+BTNYt9tUn1AI5TspZJXsrsd3dO8s6Uac=";
    };
    "x86_64-linux" = {
      url = "https://github.com/tinygo-org/tinygo/releases/download/v${version}/tinygo${version}.linux-amd64.tar.gz";
      sha256 = "sha256-is0no5CQ4eXDyjQegTUPgT7GoCv4CQxPx8Sxr9QYY0E=";
    };
    "aarch64-linux" = {
      url = "https://github.com/tinygo-org/tinygo/releases/download/v${version}/tinygo${version}.linux-arm64.tar.gz";
      sha256 = lib.fakeSha256;
    };
  };

in

stdenv.mkDerivation rec {
  pname = "tinygo";
  inherit version;

  src = fetchurl {
    inherit (sources.${stdenv.hostPlatform.system}) url sha256;
  };

  nativeBuildInputs = [ ] ++ lib.optionals (!stdenv.isDarwin) [ autoPatchelfHook ];

  installPhase = ''
    mkdir -p $out
    cp -r * $out/
  '';

  meta = with lib; {
    description = "Go compiler for small places";
    homepage = "https://tinygo.org/";
    license = licenses.bsd3;
    platforms = builtins.attrNames sources;
    maintainers = with maintainers; [ abustany ];
  };
}
