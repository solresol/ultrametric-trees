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

## Prepare

If your database after running the wordnetify programs is `w2.sqlite` and you want to create
`slm-w2.sqlite`, run:

`./bin/prepare --input-database w2.sqlite --output-database slm-w2.sqlite`

You will want to run `prepare` 4 times, to create:

- the sense annotated training data

- the sense annotated test data

- the training data without sense annotations

- the test data without sense annotations

`make prepdata` will do this for you


## Train

This creates a dataframe (in a table called `training_data`).

Then run `./bin/train --database slm-w2.sqlite`

### Advanced options

If you want to train two models at once:

`./bin/train --database slm-w2.sqlite --node-bucket model1mapping --node-table model1 --seed 1`

`./bin/train --database slm-w2.sqlite --node-bucket model2mapping --node-table model2 --seed 2`

(If you don't specify the seed, you'll end up with the same data in each model.)

### Renewable energy

At the moment, the only supported energy system is the Enphase/Envoy domestic solar system. If you
have one of these, then you can supply the argument `--solar-monitor` followed by its IP address
or hostname. If this is specified, then it will wake up every 5 minutes and confirm whether or
not production is exceeding household consumption --- only if there is spare solar power will it
attempt to do any training.

### Docker

To create the image:

`docker build -t ultratree-train -f Dockerfile.train .`

or

`make training-docker-image`

To run it, identify which directory you want to have as the directory
for the training model:

```
docker run -v /where/you/want/db/dir:/ultratree/language-model \
  ultratree-train \
  --database /ultratree/language-model/tiny.sqlite \
```

Optionally adding `--solar-monitor` to the end if relevant

## Validation

One of the weird things about the tree-based model is that we can
replay the state of the model at any time in the past, so we don't
need to capture validation loss at the end of each epoch. If we 
discover we have overfitted, we can rewind to a previous state
easily enough.

You will need:

- a trained model (which can be still training as you run this)

- a validation dataset. This is in the same format as the training data, 
  you can create it with `bin/prepare` as you otherwise would.
  
- somewhere to put the validated database
  
```
bin/validation --model tiny.sqlite \
    --validation-database validation-data.sqlite
	--output-database inference.sqlite
```

### Scheduled

Put `cronscript.sh` into a crontab to run once per day. It assumes a lot
of paths, so maybe this needs to be dockerized too.

For example, it expects ~/ultratree-results to exist and be a git repo
that it can use to store results in.


# To-do

- `cronscript.sh` should also trigger programs to graph the results, and make sure that README.md shows
  the graphs inline.

- We'll need a program that can annotate a sentence into senses. Converting the wordnetify python programs
  into golang would mostly solve that, but it would be good to have options like "manual sense annotation"
  and "sense annotation using ollama".
  
- A decoder program (it's partly done in `pkg/validation/validation.go`). Although maybe this is an `infer` program

- A path (1.3.4.1.72) should be its own type rather than a string. Not having separate types has caused a few bugs. We have half-done this with exemplar.SynsetPath

- Stats for the training and validation loss. Some sort of dashboard
  that shows the current state of training would be good too.
  
- Be able to resume training

- Parallel training (we should be able to max out every CPU comfortably)

- Training showing progress bars rather than just being silent.

- Training currently loads everything into memory. That's probably wasteful. (But it's not terrible, because
  we only do that up-front once)
  
- Random forests rather than decision trees. We need a way of saying
  "randomly select which contexts to ignore"
  
- A chatbot shell and/or CLI tool where you can ask it to complete a story.

- We should have an `<END>` marker, which probably counts as punctuation. I'm not sure if it makes sense
  when we only have a 16-word context, because we'll hopefully never hit it.
  
- More test suite

- More data

- A tool where you can ask about a particular node, and it will tell you the path taken to get there.
  I would like to double check that the same context doesn't appear twice. That should be a unit test of 
  some kind, I think

- Maybe some kind of (interactive?) tool where you can look at a predicted word, and see the reasoning

- Instructions on how to run the docker image in a kubernetes volume, or other common hosting platforms
