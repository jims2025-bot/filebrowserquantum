# Testing Guide

This project maintains unit tests for both the Backend (Go) and Frontend (Vue/JS).

## 1. Backend Tests (Go)

The backend uses the standard Go testing framework.

### How to run
Open a terminal in the project root (`filebrowserquantum`) and run:

```powershell
# Run ALL backend tests recursively
go test ./...

# Run tests for a specific package (e.g. Heatmap) with verbose output
go test ./backend/heatmap/... -v
```

### Key Test Locations
- `backend/heatmap`: Cluster logic tests.
- `backend/common/utils`: Hashing, Path parsing, Sort utils.
- `backend/common/settings`: Configuration and Default logic.
- `backend/indexing`: Index retreival logic.

## 2. Frontend Tests (Vue/JS)

The frontend uses **Vitest** for unit testing.

### How to run
Open a terminal in the `frontend` directory:

```powershell
cd frontend

# Run ALL frontend tests
npm run test

# Run a specific test file
npm run test src/utils/heatmapInspector.test.js
```

### Key Test Locations
- `src/utils/*.test.js`: Unit tests for logic extracted from Vue components (e.g. `heatmapInspector.test.js` covering Box Selection and Inspection logic).
- `src/utils`: Other utility tests.
