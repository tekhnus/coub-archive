#!/usr/bin/env bash
mkdir -p coub-archive.app/Contents/MacOS
mkdir -p coub-archive.app/Contents/Resources
cp coub-archive coub-archive.app/Contents/MacOS
cp build/package/Info.plist coub-archive.app/Contents
