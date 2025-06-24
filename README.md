# LLM Evaluation Server

This server provides functionality to test both the Gemini LLM client and the Knovvu client APIs.

## Setup

1. Create a `.env` file in the server directory with the following variables:

```
# Gemini API credentials
GEMINI_API_KEY=your_gemini_api_key_here

# Knovvu API credentials
KNOVVU_CLIENT_ID=your_knovvu_client_id_here
KNOVVU_CLIENT_SECRET=your_knovvu_client_secret_here
```

2. Install the required dependencies:

```bash
go mod tidy
```

## Running the Application

To run the application and test both clients:

```bash
go run main.go
```

The application will:
1. Test the Gemini client by sending a sample evaluation scenario
2. Test the Knovvu client by obtaining a token and sending a test message

## Expected Output

The program will output the responses from both APIs. If any environment variables are missing, the corresponding test will be skipped with an appropriate message.

## Troubleshooting

- If you encounter errors related to missing environment variables, ensure your `.env` file is properly configured
- If you encounter API errors, check your internet connection and verify that your API credentials are correct