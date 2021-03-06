/*
 * Adsisto
 * Copyright (c) 2019 Andrew Ying
 *
 * This program is free software: you can redistribute it and/or modify it under
 * the terms of version 3 of the GNU General Public License as published by the
 * Free Software Foundation. In addition, this program is also subject to certain
 * additional terms available at <SUPPLEMENT.md>.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT ANY
 * WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
 * A PARTICULAR PURPOSE.  See the GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License along with
 * this program.  If not, see <https://www.gnu.org/licenses/>.
 */

import { RECEIVE_STATE } from '../actions';

let initialState = {
  hostname: '',
  iso: undefined,
  email: '',
  accessLevel: 999,
  uuid: '',
};

export default function app(state = initialState, action) {
  switch (action.type) {
    case RECEIVE_STATE:
      return Object.assign({}, state);
    default:
      return state;
  }
}
