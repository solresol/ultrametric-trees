#!/bin/bash

# Define the URL or command to download or generate the TinyStories.sqlite file
TINY_STORIES_URL="http://example.com/path/to/TinyStories.sqlite"
OUTPUT_PATH="/tinystories/wordnetify-tinystories/TinyStories.sqlite"

# Create the directory if it doesn't exist
mkdir -p $(dirname "$OUTPUT_PATH")

# Download or generate the TinyStories.sqlite file
curl -o "$OUTPUT_PATH" "$TINY_STORIES_URL"

# Verify the download or generation was successful
if [ -f "$OUTPUT_PATH" ]; then
    echo "TinyStories.sqlite has been successfully generated or downloaded."
else
    echo "Failed to generate or download TinyStories.sqlite."
    exit 1
fi
