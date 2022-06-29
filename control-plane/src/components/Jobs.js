import React from 'react';
import JobAndLogs from './JobAndLogs';

const Jobs = (props) => {
    var rows = [];
    let jobs = props.jobs;
    for (let i = 0; i < jobs.length; i++) {
        let job = jobs[i];
        rows.push(<JobAndLogs job={job} key={job.id}/>);
    }

    return (
        <>
            {rows}
        </>
    );
}

export default Jobs