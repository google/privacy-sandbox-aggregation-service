// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import React from 'react';
import JobAndLogs from './JobAndLogs';


// The main component that controls the table of jobs
class Jobs extends React.Component {

    constructor(props) {
        super(props)
        var rows = [];
        let jobs = props.jobs;
        for (let i = 0; i < jobs.length; i++) {
            let job = jobs[i];
            rows.push(<JobAndLogs job={job} key={job.id}/>);
        }
        this.state = {
            rows: rows
        };
    }

    // when component enters DOM, add a window variable for this class property
    componentDidMount() {
        window.jobsTable = this;
    }

    // Updates state of rows in table and rerenders table
    updateJobs = (jobs) => {
        var rows = []
        for (let i = 0; i < jobs.length; i++) {
            let job = jobs[i];
            rows.push(<JobAndLogs job={job} key={job.id}/>);
        }
        this.setState({
            rows: rows
        });
    }

    render() {
        return (
            <table id="aggregation-jobs" className="mdl-data-table mdl-js-data-table mdl-data-table--selectable mdl-shadow--2dp">
                <colgroup>
                    <col width="15%" />
                    <col width="10%" />
                    <col width="45%" />
                    <col width="10%" />
                    <col width="20%" />
                </colgroup>
                <thead>
                    <tr>
                        <th className="mdl-data-table__cell--non-numeric">Created</th>
                        <th className="mdl-data-table__cell--non-numeric">Status</th>
                        <th className="mdl-data-table__cell--non-numeric">Job Id</th>
                        <th className="mdl-data-table__cell--non-numeric">Last Updated</th>
                    </tr>
                </thead>
                <tbody>
                    {this.state.rows.length > 0 ? this.state.rows : <tr><td colSpan="5"><h5 style={{textAlign : 'center' }}>No Jobs</h5></td></tr>}
                </tbody>
            </table>
        );
    }
}

export default Jobs