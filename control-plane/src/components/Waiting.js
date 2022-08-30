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

class WaitingScreen extends React.Component {

    constructor(props) {
        super(props)
    }

    // update MDL so that textfields will work
    componentDidMount() {
        componentHandler.upgradeDom();
    }

    // update MDL so that textfields will work
    componentDidUpdate() {
        componentHandler.upgradeDom();
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
                                    <li onClick={ signOutUser }>Log out</li>
                                </ul>
                            </div>
                        </div>
                    </div>
                </header>
                <main>
                    <div className='waiting-card-container'>
                        <div className="waiting-card mdl-card mdl-shadow--2dp">
                            <div className="mdl-card__title">
                                <h2 className="mdl-card__title-text">Waiting for Approval</h2>
                            </div>
                            <div className="mdl-card__supporting-text">
                                Your account is waiting for admin approval.
                                <br />
                                <br />
                                Please check back later.
                            </div>
                        </div>
                    </div>
                </main>
            </div>
        );
    }
}

export default WaitingScreen;