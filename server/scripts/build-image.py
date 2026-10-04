#!/usr/bin/env python3
"""Assemble a local OCI image from the static binary and host CA trust bundle."""
import argparse
import hashlib
import io
import json
import pathlib
import tarfile
import tempfile


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--binary", required=True)
    p.add_argument("--tag", required=True)
    p.add_argument("--output", required=True)
    args = p.parse_args()
    binary = pathlib.Path(args.binary).read_bytes()
    ca = pathlib.Path("/etc/ssl/certs/ca-certificates.crt").read_bytes()
    layer_io = io.BytesIO()
    with tarfile.open(fileobj=layer_io, mode="w", format=tarfile.PAX_FORMAT) as tar:
        for name, data, mode in [("vokirid", binary, 0o755),
                                  ("etc/ssl/certs/ca-certificates.crt", ca, 0o644)]:
            info = tarfile.TarInfo(name)
            info.size, info.mode, info.uid, info.gid, info.mtime = len(data), mode, 0, 0, 0
            tar.addfile(info, io.BytesIO(data))
    layer = layer_io.getvalue()
    with tempfile.TemporaryDirectory(prefix="vokiri-oci-") as temp:
        root = pathlib.Path(temp)
        (root / "blobs/sha256").mkdir(parents=True)

        def blob(data, media_type):
            if not isinstance(data, bytes):
                data = json.dumps(data, separators=(",", ":")).encode()
            digest = hashlib.sha256(data).hexdigest()
            (root / "blobs/sha256" / digest).write_bytes(data)
            return {"mediaType": media_type, "digest": "sha256:" + digest, "size": len(data)}

        layer_desc = blob(layer, "application/vnd.oci.image.layer.v1.tar")
        config = blob({"architecture": "amd64", "os": "linux",
                       "config": {"User": "65532:65532", "Entrypoint": ["/vokirid"],
                                  "Env": ["SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt"],
                                  "ExposedPorts": {"8680/tcp": {}}},
                       "rootfs": {"type": "layers", "diff_ids": [layer_desc["digest"]]}},
                      "application/vnd.oci.image.config.v1+json")
        manifest = blob({"schemaVersion": 2,
                         "mediaType": "application/vnd.oci.image.manifest.v1+json",
                         "config": config, "layers": [layer_desc]},
                        "application/vnd.oci.image.manifest.v1+json")
        manifest["annotations"] = {"org.opencontainers.image.ref.name": args.tag}
        manifest["platform"] = {"architecture": "amd64", "os": "linux"}
        (root / "index.json").write_text(json.dumps({"schemaVersion": 2, "manifests": [manifest]}))
        (root / "oci-layout").write_text('{"imageLayoutVersion":"1.0.0"}')
        with tarfile.open(args.output, "w") as tar:
            for f in sorted(root.rglob("*")):
                if f.is_file():
                    tar.add(f, arcname=str(f.relative_to(root)))
        print(json.dumps({"tag": args.tag, "manifest_digest": manifest["digest"],
                          "binary_sha256": hashlib.sha256(binary).hexdigest()}))


if __name__ == "__main__":
    main()
