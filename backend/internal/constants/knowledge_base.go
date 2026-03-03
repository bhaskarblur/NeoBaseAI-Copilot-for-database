package constants

// KBDescriptionGenerationPrompt is used to ask the LLM to generate table and field
// descriptions from a formatted database schema. The schema text (with examples) is
// appended as user content. The LLM returns JSON with descriptions for each table and field.
const KBDescriptionGenerationPrompt = `You are a database documentation expert. Analyze the database schema below and generate concise descriptions.

INSTRUCTIONS:
1. Analyze table structure, column types, sample data, and indexes.
2. Infer purpose from naming, types, and sample data.
3. Keep descriptions concise — 2-3 sentences for tables, 1-2 sentences for fields.
4. For nested/embedded fields (e.g. "candidate.resume.education"), only describe the TOP-LEVEL parent field (e.g. "candidate") — do NOT generate separate entries for every sub-field.
5. Focus on business meaning.

RESPONSE FORMAT — Return ONLY valid JSON:
{
  "tables": [
    {
      "table_name": "users",
      "description": "Stores user accounts and their profile information. Each record represents a registered user of the application.",
      "field_descriptions": [
        { "field_name": "id", "description": "Unique identifier for the user. Each user has a distinct ID." },
        { "field_name": "email", "description": "User's email address used for login. Used for account verification and communication." },
        { "field_name": "created_at", "description": "When the user account was created. Indicates the timestamp of account creation." }
      ]
    }
  ]
}

RULES:
- Include ALL tables but only TOP-LEVEL fields (max 2 levels of nesting).
- Do NOT generate entries for deeply nested sub-fields like "candidate.resume.education.location.city".
- Do NOT invent tables or fields.
- Return pure JSON only — no markdown, no explanation text.`
