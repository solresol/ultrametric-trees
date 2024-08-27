# ultrametric-trees

Training decision trees that minimise an ultrametric

There's a lot of depedency on the wordnetify project still. When you have run everything in that
project, you will have a sqlite database with a large table called `words`, with `resolved_synset` 
as one of its columns. There will also be a table `synset_paths` that is approximately the paths
that we need to work with here.

# Building this project

Install a golang compiler and something that reads Makefiles. Type `make` and it should build
everything you need.

# Running

If your database after running the wordnetify programs is `w2.sqlite` and you want to create
`slm-w2.sqlite`, run:

`./bin/prepare --input-database w2.sqlite --output-database slm-w2.sqlite`

This creates a dataframe (in a table called `training_data`).

Then run `./bin/train --database slm-w2.sqlite`

# To-do

- We'll need a program that can annotate a sentence into senses. Converting the wordnetify python programs
  into golang would mostly solve that, but it would be good to have options like "manual sense annotation"
  and "sense annotation using ollama".
  
- A decoder program

- Stats for the training and validation loss. One of the weird things about the tree-based model is that
  we can replay the state of the model at any time in the past, so we don't need to capture validation
  loss at the end of each epoch. Some sort of dashboard that shows the current state of training would be
  good too.
  
- Be able to resume training

- Parallel training (we should be able to max out every CPU comfortably)

- Training showing progress bars rather than just being silent.

- Training currently loads everything into memory. That's probably wasteful. (But it's not terrible, because
  we only do that up-front once)
  
- Random forests rather than decision trees. Currently the node that a row belongs to is in the dataframe
  itself. It should be in a separate table, so that we can be training multiple trees concurrently.
  
- A chatbot shell and/or CLI tool where you can ask it to complete a story.

- We should have an `<END>` marker, which probably counts as punctuation. I'm not sure if it makes sense
  when we only have a 16-word context, because we'll hopefully never hit it.
  
- More test suite

- More data


