import React from 'react';
import Job from './Job';
import Logs from './Logs';

const JobAndLogs = (props) => {
    return (
        <>
            <Job job={props.job} />
            <Logs job={props.job} />
        </>
    )   
}

export default JobAndLogs;