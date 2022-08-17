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
import { signInWithPopup, GoogleAuthProvider, GithubAuthProvider, signInWithEmailAndPassword } from "firebase/auth";
import VALUES from "../values";

function Login({ parent }) {

    function updatePage() {
        parent.setState( { login: false } )
    }

    function googleLogin() {
        const provider = new GoogleAuthProvider();
        signUserIn(provider)
    }

    function githubLogin() {
        const provider = new GithubAuthProvider();
        signUserIn(provider)
    }

    function signUserIn(provider) {
        if(VALUES.auth != null && VALUES.db != null) {
            signInWithPopup(VALUES.auth, provider)
                .then(async (result) => {
                    // The signed-in user info.
                    const user = result.user;
                    VALUES.user = user;
                }).catch((error) => {
                    // Handle Errors here.
                    const errorCode = error.code;
                    const errorMessage = error.message;
                    console.error("Failed with error code: "+errorCode+". Message: "+errorMessage)
                });
        }
    }

    function emailPasswordLogin() {
        var email = $('#email').val();
        var password = $("#password").val();
        signInWithEmailAndPassword(VALUES.auth, email, password)
            .then((userCredential) => {
                const user = userCredential.user;
                VALUES.user = user;
            })
            .catch((error) => {
                const errorCode = error.code;
                const errorMessage = error.message;
                console.error("Failed with error code: "+errorCode+". Message: "+errorMessage)
            });
    }

    return (
        <div>
            <img src="./images/controlplane.png" id="main-icon" />
            <h3>Aggregation Jobs Control Plane</h3>
            <div className="social-logins" id="social-logins">
                <button onClick={ googleLogin }>
                    <img src="https://developers.google.com/static/identity/images/g-logo.png"/><span>Sign in with Google</span>
                </button>
                <button onClick={ githubLogin }>
                    <div className='github-logo-crop'>
                        <img src="https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png" className='github-logo' />
                    </div>
                    <span>Sign in with Github</span>
                </button>
            </div>
            <br />
            <div className="or-text-container">
                <p className="or-text">OR</p>
            </div>
            <div className="mdl-textfield mdl-js-textfield mdl-textfield--floating-label">
                <input className="mdl-textfield__input" type="text" id="email" />
                <label className="mdl-textfield__label" htmlFor="email">Email</label>
            </div>
            <br />
            <div className="mdl-textfield mdl-js-textfield mdl-textfield--floating-label">
                <input className="mdl-textfield__input" type="password" id="password" />
                <label className="mdl-textfield__label" htmlFor="password">Password</label>
            </div>
            <br />
            <button onClick={ emailPasswordLogin } className="mdl-button mdl-js-button mdl-button--raised mdl-button--colored login-button" type='button'>
                Login
            </button>
            <br />
            <p className="move-to">No account? <a href="#" onClick={ updatePage }>Create One</a></p>
        </div>
    );
}

export default Login;