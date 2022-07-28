import React from 'react';
import VALUES from '../values'
import { formatTime } from '../jobs-functions';

// Job component for the table
const Job = (props) => {
    let job = props.job;

    return (
        <tr id={job.id} className="job" data-status={job.status}>
            <td className="mdl-data-table__cell--non-numeric">{formatTime(job.created)}</td>
            <td className="mdl-data-table__cell--non-numeric"><i className={"material-icons " + job.status}>{VALUES.jobStatusIcons[job.status]}</i></td>
            <td className="mdl-data-table__cell--non-numeric">{job.id}</td>
            <td className="mdl-data-table__cell--non-numeric">{formatTime(job.updated, true)}</td>
            <td>
                <i className="material-icons" id="delete-job" data-id={job.id}>delete</i>
                <i className="material-icons" data-status={job.status} id={job.id + "-dropdown"}>keyboard_arrow_down</i>
            </td>
        </tr>
    );
}

export default Job;