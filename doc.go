/*
Package ratchet is a library for performing data pipeline / ETL tasks in Go.

The main construct in Ratchet is Pipeline. A Pipeline has a series of
PipelineStages, which will each perform some type of data processing, and
then send new data on to the next stage. Each PipelineStage consists of one
or more DataProcessors, which are responsible for receiving, processing, and
then sending data on to the next stage of processing. DataProcessors each
run in their own goroutine, and therefore all data processing can be executing
concurrently.

Here is a conceptual drawing of a fairly simple Pipeline:

        +--Pipeline------------------------------------------------------------------------------------------+
        |                                                                       PipelineStage 3              |
        |                                                                      +---------------------------+ |
        |  PipelineStage 1                 PipelineStage 2          +-JSON---> |  CSVWriter                | |
        | +------------------+           +-----------------------+  |          +---------------------------+ |
        | |  SQLReader       +-JSON----> | Custom DataProcessor  +--+                                        |
        | +------------------+           +-----------------------+  |          +---------------------------+ |
        |                                                           +-JSON---> |  SQLWriter                | |
        |                                                                      +---------------------------+ |
        +----------------------------------------------------------------------------------------------------+

In this example, we have a Pipeline consisting of 3 PipelineStages. The first stage has a DataProcessor that
runs queries on a SQL database, the second is doing custom transformation
work on that data, and the third stage branches into 2 DataProcessors, one
writing the resulting data to a CSV file, and the other inserting into another
SQL database.

In the example above, Stage 1 and Stage 3 are using built-in DataProcessors
(see the "processors" package/subdirectory). However, Stage 2 is using a custom
implementation of DataProcessor. By using a combination of built-in processors,
and supporting the writing of any Go code to process data, Ratchet makes
it possible to write very custom and fast data pipeline systems. See the
DataProcessor documentation to learn more.

Since each DataProcessor is running in it's own goroutine, SQLReader can continue pulling and sending
data while each subsequent stage is also processing data. Optimally-designed pipelines
have processors that can each run in an isolated fashion, processing data without having
to worry about what's coming next down the pipeline.

All data payloads sent between DataProcessors are of type data.JSON ([]byte). This provides
a good balance of consistency and flexibility. See the "data" package for details
and helper functions for dealing with data.JSON. Another good read for handling
JSON data in Go is http://blog.golang.org/json-and-go.

Note that many of the concepts in Ratchet were taken from the Golang blog's post on
pipelines (http://blog.golang.org/pipelines). While the details discussed in that
blog post are largely abstracted away by Ratchet, it is still an interesting read and
will help explain the general concepts being applied.

Creating and Running a Basic Pipeline

There are two ways to construct and run a Pipeline. The first is a basic, non-branching
Pipeline. For example:

        +------------+   +-------------------+   +---------------+
        | SQLReader  +---> CustomTransformer +---> SQLWriter     |
        +------------+   +-------------------+   +---------------+

This is a 3-stage Pipeline that queries some SQL data in stage 1, does some custom data
transformation in stage 2, and then writes the resulting data to a SQL table in stage 3.
The code to create and run this basic Pipeline would look something like:

        // First initalize the DataProcessors
        read := processors.NewSQLReader(db1, "SELECT * FROM source_table")
        transform := NewCustomTransformer() // (This would your own custom DataProcessor implementation)
        write := processors.NewSQLWriter(db2, "destination_table")

        // Then create a new Pipeline using them
        pipeline := ratchet.NewPipeline(read, transform, write)

        // Finally, run the Pipeline and wait for either an error or nil to be returned
        err := <-pipeline.Run()

Creating and Running a Branching Pipeline

The second way to construct a Pipeline is using a PipelineLayout. This method allows
for more complex Pipeline configurations that support branching between stages that
are running multiple DataProcessors. Here is a (fairly complex) example:

                                                                   +----------------------+
                                                            +------> SQLReader (Dynamic)  +--+
                                                            |      +----------------------+  |
                                                            |                                |
                         +---------------------------+      |      +----------------------+  |    +-----------+
                   +-----> SQLReader (Dynamic Query) +------+   +--> Custom DataProcessor +-------> CSVWriter |
    +-----------+  |     +---------------------------+      |   |  +----------------------+  |    +-----------+
    | SQLReader +--+                                     +------+                            |
    +-----------+  |     +---------------------------+   |  |      +----------------------+  |    +-----------+
                   +-----> Custom DataProcessor      +------+------> Custom DataProcessor +--+  +-> SQLWriter |
                         +---------------------------+   |         +----------------------+     | +-----------+
                                                         |                                      |
                                                         |         +----------------------+     |
                                                         +---------> Passthrough          +-----+
                                                                   +----------------------+

This Pipeline consists of 4 stages where each DataProcessor is choosing which DataProcessors
in the subsequent stage should receive the data it sends. The SQLReader in stage 2, for example,
is sending data to only 2 processors in the next stage, while the Custom DataProcessor in
stage 2 is sending it's data to 3. The code for constructing and running a Pipeline like this
would look like:

        // First, initialize all the DataProcessors that will be used in the Pipeline
        query1 := processors.NewSQLReader(db1, "SELECT * FROM source_table")
        query2 := processors.NewSQLReader(db1, sqlGenerator1) // sqlGenerator1 would be a function that generates the query at run-time. See SQLReader docs.
        custom1 := NewCustomDataProcessor1()
        query3 := processors.NewSQLReader(db2, sqlGenerator2)
        custom2 := NewCustomDataProcessor2()
        custom3 := NewCustomDataProcessor3()
        passthrough := processors.NewPassthrough()
        writeMySQL := processors.NewSQLWriter(db3, "destination_table")
        writeCSV := processors.NewCSVWriter(file)

        // Next, construct and validate the PipelineLayout. Each DataProcessor
        // is inserted into the layout via calls to ratchet.Do().
        layout, err := ratchet.NewPipelineLayout(
                ratchet.NewPipelineStage(
                        ratchet.Do(query1).Outputs(query2),
                        ratchet.Do(query1).Outputs(custom1),
                ),
                ratchet.NewPipelineStage(
                        ratchet.Do(query2).Outputs(query3, custom3),
                        ratchet.Do(custom1).Outputs(custom2, custom3, passthrough),
                ),
                ratchet.NewPipelineStage(
                        ratchet.Do(query3).Outputs(writeCSV),
                        ratchet.Do(custom2).Outputs(writeCSV),
                        ratchet.Do(custom3).Outputs(writeCSV),
                        ratchet.Do(passthrough).Outputs(writeMySQL),
                ),
                ratchet.NewPipelineStage(
                        ratchet.Do(writeCSV),
                        ratchet.Do(writeMySQL),
                ),
        )
        if err != nil {
                // layout is invalid
                panic(err.Error())
        }

        // Finally, create and run the Pipeline
        pipeline := ratchet.NewBranchingPipeline(layout)
        err = <-pipeline.Run()

This example is only conceptual, the main points being to explain the flexibility
you have when designing your Pipeline's layout and to demonstrate the syntax for
constructing a new PipelineLayout.

*/
package ratchet
