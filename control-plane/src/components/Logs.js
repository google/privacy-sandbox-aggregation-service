import React from 'react';
import Log from './Log';

// Component that controls all subjobs in a job
const Logs = (props) => {
    var rows = [];
    let job = props.job;
    for (let i = 0; i < job.logs.length; i++) {
        let log = job.logs[i];
        rows.push(<Log job={job} log={log} key={i} />);
    }
    return (
        <tr id={job.id + "-info"} className="logs">
            <td colSpan={5}>
                <div className="levels-logs">
                    <h5>Sub-job Status</h5>
                    {rows}
                </div>
            </td>
        </tr>
    )
}

export default Logs;