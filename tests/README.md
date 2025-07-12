# RAG CLI Test Suite

This directory contains modular tests for the RAG CLI application.

## Structure

- `run_all.sh` - Main test runner that executes all test files
- `test_*.sh` - Individual test files for specific functionality

## Running Tests

To run all tests:
```bash
./tests/run_all.sh
```

To run a specific test:
```bash
./tests/test_01_basic_command.sh
```

## Test Files

1. **test_01_basic_command.sh** - Basic chat command execution
2. **test_02_system_detection.sh** - System detection and platform-aware commands
3. **test_03_tools_detection.sh** - Available tools detection
4. **test_04_multistep_execution.sh** - Multi-step command execution with learning
5. **test_05_error_recovery.sh** - Error recovery and learning
6. **test_06_file_processing.sh** - File processing, chunking, embeddings, and ChromaDB
7. **test_07_goal_recognition.sh** - Command evaluation and goal recognition
8. **test_08_auto_indexing.sh** - Auto-indexing of file changes

## Adding New Tests

To add a new test:

1. Create a new file following the naming pattern `test_##_description.sh`
2. Make it executable with `chmod +x tests/test_##_description.sh`
3. The test runner will automatically detect and run it

## Test Structure

Each test file should:
- Use `create_temp_file()` for temporary output files
- Exit with code 0 for success, non-zero for failure
- Clean up any files it creates (except temp files which are auto-cleaned)
- Use descriptive echo statements for progress

## Temporary Files

The test runner creates a temporary directory (`$TEST_TEMP_DIR`) and provides a helper function `create_temp_file()` that creates temporary files in this directory. All temporary files are automatically cleaned up when the test runner exits.
