#!/usr/bin/env python3
"""Apply the source-bound Ebitengine 2.9.9 Linux XIM edit-key fix."""
import hashlib
from pathlib import Path

path = Path("vendor/github.com/hajimehoshi/ebiten/v2/internal/glfw/x11_window_linbsd.c")
source_hash = "eaba4f2e10664d997a32386f1dfdf79adbce5b2c51e57b2f466ef2d334c00270"
patched_hash = "149f76078c8b284afab13c8706d59b2d2243d5e888172f80ac6e375a9228d623"
old = """                    if (keycode)
                        _glfwInputKey(window, key, keycode, GLFW_PRESS, mods);

                    window->x11.keyPressTimes[keycode] = event->xkey.time;"""
new = """                    // MM2: XIM owns edit keys it consumed while composing.
                    // Keep physical gameplay keys and all unfiltered events.
                    const int imeEdit = key == GLFW_KEY_ENTER ||
                                        key == GLFW_KEY_KP_ENTER ||
                                        key == GLFW_KEY_ESCAPE ||
                                        key == GLFW_KEY_BACKSPACE ||
                                        key == GLFW_KEY_DELETE;
                    if (!(filtered && imeEdit))
                    {
                        if (keycode)
                            _glfwInputKey(window, key, keycode, GLFW_PRESS, mods);
                        // An unhandled XIM event may be forwarded later with
                        // the same timestamp. Mark only events we reported.
                        window->x11.keyPressTimes[keycode] = event->xkey.time;
                    }"""
blob = path.read_bytes()
if new.encode() in blob:
    assert hashlib.sha256(blob).hexdigest() == patched_hash, "modified patched GLFW source"
    print("XIM patch already present")
else:
    assert hashlib.sha256(blob).hexdigest() == source_hash, "unexpected Ebitengine GLFW source"
    assert blob.count(old.encode()) == 1
    path.write_bytes(blob.replace(old.encode(), new.encode(), 1))
    assert hashlib.sha256(path.read_bytes()).hexdigest() == patched_hash
    print("Applied XIM edit-key patch:", hashlib.sha256(path.read_bytes()).hexdigest())
