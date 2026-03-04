#!/usr/bin/env python3
"""Convert legacy langid.py model files into Go .lidg binary format."""

from __future__ import annotations

import argparse
import array
import base64
import bz2
import io
import pickle
import struct
import warnings
from pathlib import Path

MAGIC = b"LIDG1\x00"


def compat_array(typecode, initializer=None):
    if isinstance(typecode, bytes):
        typecode = typecode.decode("latin1")
    out = array.array(typecode)
    if initializer is None:
        return out
    if isinstance(initializer, str):
        initializer = initializer.encode("latin1")
    if isinstance(initializer, (bytes, bytearray)):
        out.frombytes(initializer)
    else:
        out.extend(initializer)
    return out


class CompatUnpickler(pickle.Unpickler):
    def find_class(self, module, name):
        if module == "array" and name == "array":
            return compat_array
        return super().find_class(module, name)


def load_legacy_model(path: Path):
    raw = path.read_bytes()
    payload = bz2.decompress(base64.b64decode(raw))
    # Old pickles can trigger NumPy compatibility warnings during decode.
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        model = CompatUnpickler(io.BytesIO(payload), encoding="latin1").load()
    nb_ptc, nb_pc, nb_classes, tk_nextmove, tk_output = model

    num_langs = len(nb_pc)
    num_feats = len(nb_ptc) // num_langs
    num_states = len(tk_nextmove) // 256

    tk_output_c = [0] * num_states
    tk_output_s = [0] * num_states
    tk_output_flat = []

    for state in range(num_states):
        feats = tk_output.get(state, [])
        tk_output_c[state] = len(feats)
        tk_output_s[state] = len(tk_output_flat)
        tk_output_flat.extend(int(x) for x in feats)

    classes = []
    for c in nb_classes:
        if isinstance(c, bytes):
            classes.append(c.decode("utf-8"))
        else:
            classes.append(str(c))

    return {
        "num_feats": num_feats,
        "num_langs": num_langs,
        "num_states": num_states,
        "tk_nextmove": tk_nextmove,
        "tk_output_c": tk_output_c,
        "tk_output_s": tk_output_s,
        "tk_output": tk_output_flat,
        "nb_pc": nb_pc,
        "nb_ptc": nb_ptc,
        "classes": classes,
    }


def write_model(model, out_path: Path):
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with out_path.open("wb") as w:
        w.write(MAGIC)
        w.write(struct.pack("<III", model["num_feats"], model["num_langs"], model["num_states"]))

        tk_nextmove = array.array("H", model["tk_nextmove"])
        w.write(tk_nextmove.tobytes())

        tk_output_c = array.array("H", model["tk_output_c"])
        w.write(tk_output_c.tobytes())

        tk_output_s = array.array("I", model["tk_output_s"])
        w.write(tk_output_s.tobytes())

        tk_output = array.array("I", model["tk_output"])
        w.write(struct.pack("<I", len(tk_output)))
        w.write(tk_output.tobytes())

        nb_pc = array.array("f", model["nb_pc"])
        w.write(nb_pc.tobytes())

        nb_ptc = array.array("f", model["nb_ptc"])
        w.write(nb_ptc.tobytes())

        for c in model["classes"]:
            b = c.encode("utf-8")
            if len(b) > 65535:
                raise ValueError("class label too long")
            w.write(struct.pack("<H", len(b)))
            w.write(b)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("input", type=Path, help="legacy langid model (base64+bz2+pickle)")
    p.add_argument("output", type=Path, help="output .lidg file")
    args = p.parse_args()

    model = load_legacy_model(args.input)
    write_model(model, args.output)


if __name__ == "__main__":
    main()
