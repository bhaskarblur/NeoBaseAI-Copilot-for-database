#!/bin/bash

# Test Google Sheets connection creation
echo "Testing Google Sheets Integration..."

# Get auth token (you should have one from previous login)
TOKEN="YOUR_AUTH_TOKEN"

# Test creating a Google Sheets connection
curl -X POST http://localhost:3001/api/chats \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "connection": {
      "type": "google_sheets",
      "database": "google_sheets_test",
      "google_sheet_id": "YOUR_SHEET_ID",
      "google_auth_token": "YOUR_GOOGLE_AUTH_TOKEN",
      "google_refresh_token": "YOUR_GOOGLE_REFRESH_TOKEN"
    },
    "settings": {
      "auto_execute_query": true,
      "share_data_with_ai": false
    }
  }'