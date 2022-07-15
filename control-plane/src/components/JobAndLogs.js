import React from 'react';
import Job from './Job';
import Logs from './Logs';

// component for both the job and the subjobs beneath
const JobAndLogs = (props) => {
    return (
        <>
            <Job job={props.job} key={props.job.id + "-job" } />
            <Logs job={props.job} key={props.job.id + "-logs"} />
        </>
    )   
}

export default JobAndLogs;