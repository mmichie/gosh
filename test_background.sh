#!/bin/bash
# Test script for background pipeline execution

echo "Testing background pipeline execution"

# Keep tests running smoothly without debug output

# Create a test file to filter
echo "Line 1 - will match" > test_input.txt
echo "Line 2 - no match" >> test_input.txt
echo "Line 3 - will match" >> test_input.txt

# Test 1: Run a simple background command
echo -e "\n\n===== TEST 1: Simple background command ====="
./bin/gosh -c "sleep 3 &"
echo "If the command runs in background, this should print immediately"
sleep 5  # Wait for the background process to finish

# Test 2: Run a pipeline in background
echo -e "\n\n===== TEST 2: Pipeline background execution ====="
./bin/gosh -c "cat test_input.txt | grep 'match' &"
echo "If the pipeline runs in background, this should print immediately"
sleep 5  # Wait for the background process to finish

# Test 3: Run a complex pipeline with redirection in background
echo -e "\n\n===== TEST 3: Complex pipeline with redirection in background ====="
./bin/gosh -c "cat test_input.txt | grep 'match' > output.txt &"
echo "If the complex pipeline runs in background, this should print immediately"
sleep 5  # Wait for the background process to finish

# Clean up
rm -f test_input.txt output.txt

echo -e "\n\nAll tests completed"