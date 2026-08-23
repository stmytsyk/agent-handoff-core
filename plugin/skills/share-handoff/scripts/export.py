#!/usr/bin/env python3
import argparse
import json
import socket


SOCKET_PATH = "/tmp/ahp.sock"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", required=True)
    args = parser.parse_args()

    request = {"command": "share", "target": args.target}
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
        client.connect(SOCKET_PATH)
        client.sendall(json.dumps(request).encode("utf-8") + b"\n")
        response = client.recv(10 * 1024 * 1024)
    print(response.decode("utf-8"))


if __name__ == "__main__":
    main()

