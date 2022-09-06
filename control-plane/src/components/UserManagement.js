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
import VALUES from '../values';
import { signOutUser } from '../auth-functions';
import User from './User';
import PendingUser from './PendingUser';
import { query, limit,  collection, getDocs } from "firebase/firestore";

class UserManagement extends React.Component {

    constructor(props) {
        super(props);
        this.state = {
            users: [],
            type: "Users"
        };
    }

    // update MDL so that textfields will work
    componentDidMount() {
        this.showUsers();
        componentHandler.upgradeDom();
    }

    // update MDL so that textfields will work
    componentDidUpdate() {
        componentHandler.upgradeDom();
    }

    showUsers = async () => {
        let users = [];
        const querySnapshot = await getDocs(query(collection(VALUES.db, "user-management", "users", "existing-users"), limit(10)));
        await Promise.all(querySnapshot.docs.map(async (doc) => {
            let user = {};
            user.id = doc.id;
            const vals = doc.data();
            user.created = vals.time;
            user.email = vals.email;
            user.role = vals.role;
            users.push(<User user={user} parent={this} key={user.id} />);
        }));
        this.setState({
            users: users,
            type: "Users"
        });
    }

    showPendingUsers = async () => {
        let users = [];
        const querySnapshot = await getDocs(query(collection(VALUES.db, "user-management", "users", "pending-users"), limit(10)));
        await Promise.all(querySnapshot.docs.map(async (doc) => {
            let user = {};
            user.id = doc.id;
            const vals = doc.data();
            user.created = vals.time;
            user.email = vals.email;
            users.push(<PendingUser user={user} parent={this} key={user.id} />);
        }));
        this.setState({
            users: users,
            type: "Pending Users"
        });
    }

    render() {
        return (
            <div className="mdl-layout mdl-js-layout mdl-layout--fixed-drawer mdl-layout--fixed-header">
                <header className="mdl-layout__header">
                    <div className="mdl-layout__header-row">
                        <span className="mdl-layout-title">Job Control Plane</span>
                        <div className="mdl-layout-spacer"></div>
                        <div className='user-image'>
                        {VALUES.user.photoURL != null ? <img src={VALUES.user.photoURL} /> : <i className='material-icons'>account_circle</i>}
                            <div className='user-account-dropdown'>
                                <ul>
                                    <li onClick={signOutUser}>Log out</li>
                                </ul>
                            </div>
                        </div>
                    </div>
                </header>
                <div className="mdl-layout__drawer">
                    <span className="mdl-layout-title">User Management</span>
                    <nav className="mdl-navigation">
                        <a className="mdl-navigation__link" href="#" onClick={ this.showUsers }>Users</a>
                        <a className="mdl-navigation__link" href="#" onClick={ this.showPendingUsers }>Pending Users</a>
                        <a className="mdl-navigation__link" href="">Return Home</a>
                    </nav>
                </div>
                <main className="mdl-layout__content">
                    <div className="users-table-container">
                        <h3>{ this.state.type }</h3>
                        <br />
                        <table className="mdl-data-table mdl-js-data-table mdl-shadow--2dp users-table">
                            <colgroup>
                                <col width="15%" />
                                <col width="10%" />
                                <col width="35%" />
                                <col width="20%" />
                                <col width="20%" />
                            </colgroup>
                            <thead>
                                <tr>
                                    <th className="mdl-data-table__cell--non-numeric">Created</th>
                                    <th className="mdl-data-table__cell--non-numeric">Role</th>
                                    <th className="mdl-data-table__cell--non-numeric">User Id</th>
                                    <th className="mdl-data-table__cell--non-numeric">Email</th>
                                </tr>
                            </thead>
                            <tbody>
                                {this.state.users.length > 0 ? this.state.users : <tr><td colSpan="5"><h5 style={{textAlign : 'center' }}>No Users</h5></td></tr>}
                            </tbody>
                        </table>
                    </div>
                </main>
            </div>
        );
    }
}

export default UserManagement;