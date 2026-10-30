import type fr from './fr'

export default {
  app: { name: 'Toto', tagline: 'Your personal film library' },
  common: {
    username: 'Username',
    password: 'Password',
    language: 'Language',
    logout: 'Sign out',
    loading: 'Loading…',
  },
  setup: {
    title: 'Welcome',
    subtitle: 'Create the administrator account to get started. It can then add libraries and other users.',
    confirmPassword: 'Confirm password',
    submit: 'Create administrator account',
    passwordHint: 'At least 8 characters',
    usernameHint: "3 to 32 characters: letters, digits, '.', '_' or '-'",
  },
  login: {
    title: 'Sign in',
    subtitle: 'Sign in to access your libraries.',
    submit: 'Sign in',
  },
  home: {
    continueWatching: 'Continue watching',
    nextUp: 'Next up',
    libraries: 'Libraries',
    emptyLibraries: 'No libraries yet.',
    emptyLibrariesAdmin: 'Add a library from the administration to start discovering your videos.',
    emptyContinue: 'Nothing in progress.',
    emptyNextUp: 'No episodes to watch next.',
  },
  nav: { home: 'Home', admin: 'Administration' },
  errors: {
    passwordMismatch: 'Passwords do not match.',
    invalid_credentials: 'Invalid username or password.',
    username_taken: 'This username is already taken.',
    already_initialized: 'The application is already initialized.',
    validation: 'Some fields are invalid.',
    network: 'The server is unreachable.',
    generic: 'Something went wrong.',
  },
} satisfies typeof fr
