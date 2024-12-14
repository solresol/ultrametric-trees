   Update the path in the Makefile if your file is located elsewhere.

## Verifying the File Path

To verify the file path:

1. Open a terminal and navigate to the directory where the `TinyStories.sqlite` file is supposed to be located.
2. Use the `ls` command to list files and ensure `TinyStories.sqlite` is present:
   ```
   ls /correct/path/to/
   ```
3. If the file is not present, double-check the download location and move the file to the correct directory.

## Build Process

Once the prerequisites are met and the `TinyStories.sqlite` file is correctly placed, follow these steps to build the project:

1. **Clone the Repository**:
   ```
   git clone https://github.com/solresol/ultrametric-trees.git
   cd ultrametric-trees
   ```

2. **Run the Build Command**:
   Use the Makefile to build the necessary binaries: