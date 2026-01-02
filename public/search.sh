#!/bin/bash
# search.sh - Migrated from Locust to SteadyQ Script Mode
# Usage: ./search.sh {{userID}} {{uuid}}

QUESTIONS=(
    "What is rainwater harvesting?"
    "Explain POSH act"
    "How does BigQuery work?"
    "What is RAG in AI?"
    "Explain vector search"
    "What is Karmayogi Bharat?"
    "How does Gemini LLM work?"
    "Explain cosine similarity"
    "What is cloud storage?"
    "What is an embedding model?"
)

# Pick a random question
RANDOM_Q=${QUESTIONS[$RANDOM % ${#QUESTIONS[@]}]}

# $1 = userID, $2 = uuid (passed by SteadyQ)
# Update the URL below to your actual endpoint
TARGET="https://learning-ai.prod.karmayogibharat.net/api/kb-pipeline/v3/search"

curl -s -X POST "$TARGET?userID=$1&chatID=$2" \
     -H "Content-Type: application/json" \
     -d "{\"query\": \"$RANDOM_Q\"}"
