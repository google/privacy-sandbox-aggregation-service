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
import Login from './Login';
import SignUp from './Signup';

// The main component that controls Authentication
class Authentication extends React.Component {

    constructor(props) {
        super(props)
        this.state = {
            login: true,
        }
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
            <div className="mdl-layout mdl-js-layout">
                <main className="mdl-layout__content">
                    <div className="auth-screen">
                        <div className="auth-block">
                            <div className="center-auth-items">
                                {this.state.login ? <Login parent={this} /> : <SignUp parent={this} />}
                            </div>
                        </div>
                    </div>
                </main>
            </div>
        );
    }
}

export default Authentication;