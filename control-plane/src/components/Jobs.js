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
import VALUES from '../values';
import { showUserManagementScreen, signOutUser } from '../auth-functions';
import { filterJobs } from '../jobs-functions';

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
        componentHandler.upgradeDom();
        $('#p2').hide();
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
        $('#p2').hide();
    }

    render() {
        return (
            <div className="mdl-layout mdl-js-layout
                mdl-layout--fixed-header">
                <header className="mdl-layout__header">
                    <div className="mdl-layout__header-row">
                        <span className="mdl-layout-title">Job Control Plane</span>
                        <div className="mdl-layout-spacer"></div>
                        <div className='user-image'>
                            {VALUES.user.photoURL != null ? <img src={VALUES.user.photoURL} /> : <i className='material-icons'>account_circle</i>}
                            <div className='user-account-dropdown'>
                                <ul>
                                    <li onClick={ showUserManagementScreen }>Users</li>
                                    <li onClick={ signOutUser }>Log out</li>
                                </ul>
                            </div>
                        </div>
                    </div>
                </header>
                <main className="mdl-layout__content">
                    <div className="page-content">

                        <div className="table-section">
                            <div className="page-header">
                                <h3>Aggregation Table</h3><span id="filter-jobs" className="filter-jobs"><i className="material-icons">filter_list</i></span>
                            </div>
            
                            <div style={{ textAlign: 'center' }}>
                                <div id="p2" className="mdl-progress mdl-js-progress mdl-progress__indeterminate"></div>
                            </div>
                            <div id="render-table">
                                <table id="aggregation-jobs" className="mdl-data-table mdl-js-data-table mdl-shadow--2dp">
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
                            </div>
                            <div className="pages">
                                <span>1</span>
                                <button id="prev-page" className="mdl-button mdl-js-button mdl-button--icon">
                                    <i className="material-icons">navigate_before</i>
                                </button>
                                <button id="next-page" className="mdl-button mdl-js-button mdl-button--icon">
                                    <i className="material-icons">navigate_next</i>
                                </button>
                            </div>
                            
                            <div id="snackbar-container" className="mdl-js-snackbar mdl-snackbar">
                                <div className="mdl-snackbar__text"></div>
                                <button className="mdl-snackbar__action" type="button"></button>
                            </div>
                        </div>
                        <div className="filter-side-holder">
                            <h5>Job Id</h5>
                            <div class="mdl-textfield mdl-js-textfield">
                                <input class="mdl-textfield__input" type="text" id="search" />
                                <label class="mdl-textfield__label" for="search">Search</label>
                            </div>
                            <h5>Status</h5>
                            <div className="mdl-textfield mdl-js-textfield mdl-textfield--floating-label">
                                <select className="mdl-textfield__input" id="status" name="status">
                                <option></option>
                                <option value="running">Running</option>
                                <option value="scheduled">Scheduled</option>
                                <option value="failed">Failed</option>
                                <option value="finished">Finished</option>
                                </select>
                                <label className="mdl-textfield__label" htmlFor="status">Status</label>
                            </div>
                            <br />
                            <h5>Created</h5>
                            <div className="mdl-textfield mdl-js-textfield mdl-textfield--floating-label">
                                <select className="mdl-textfield__input" id="created" name="created">
                                    <option></option>
                                    <option value="1">1 hour</option>
                                    <option value="2">2 hours</option>
                                    <option value="6">6 hours</option>
                                    <option value="12">12 hours</option>
                                    <option value="24">1 day</option>
                                    <option value="120">5 days</option>
                                    <option value="240">10 days</option>
                                    <option value="720">1 month</option>
                                    <option value="4320">6 months</option>
                                </select>
                                <label className="mdl-textfield__label" htmlFor="created">Created</label>
                            </div>
                            <br />
                            <h5>Last Updated</h5>
                            <div className="mdl-textfield mdl-js-textfield mdl-textfield--floating-label">
                                <select className="mdl-textfield__input" id="updated" name="updated">
                                <option></option>
                                <option value="1">1 hour</option>
                                <option value="2">2 hours</option>
                                <option value="6">6 hours</option>
                                <option value="12">12 hours</option>
                                <option value="24">1 day</option>
                                <option value="120">5 days</option>
                                <option value="240">10 days</option>
                                <option value="720">1 month</option>
                                <option value="4320">6 months</option>
                                </select>
                                <label className="mdl-textfield__label" htmlFor="updated">Last Updated</label>
                            </div>
                            <p id="updated-created-error" className="error-message">Last updated and created can't both be set</p>
                            <br />
                            <div style={{ textAlign: "left" }}>
                                <button id="filter-jobs-button" className="mdl-button mdl-js-button mdl-button--raised mdl-button--colored mdl-js-ripple-effect" type="button" onClick={ filterJobs }>
                                    Go
                                </button>
                            </div>
                        </div>
                    </div>
                    <br />
                    <br />
                    <br />
                </main>
            </div>
        );
    }
}

export default Jobs;