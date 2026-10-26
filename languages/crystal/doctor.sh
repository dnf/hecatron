#!/usr/bin/env bash
# Check for shards, which the Crystal suites install their (empty) deps with.
source "$(dirname "$0")/../_common.sh"

has shards || problem "shards not found"
doctor_done
