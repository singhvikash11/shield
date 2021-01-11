import Confidence from 'confidence';

const internals: any = {
  criteria: {
    env: process.env.NODE_ENV,
    ci: process.env.CI
  }
};

internals.config = {
  $meta: 'App configuration',
  env: {
    $filter: 'env',
    production: 'production',
    integration: 'integration',
    test: 'test',
    $default: 'dev'
  },
  port: {
    web: {
      $filter: 'env',
      test: 9000,
      production: process.env.PORT,
      integration: process.env.PORT,
      $default: process.env.PORT || 8000
    }
  },
  postgres: {
    $filter: 'env',
    test: {
      $filter: 'ci',
      gitlab: {
        uri: 'postgresql://shield_test@localhost:4322/shield_test'
      },
      $default: {
        uri: 'postgresql://shield_test@localhost:4322/shield_test'
      }
    },
    $default: {
      uri: process.env.POSTGRES_HOST,
      options: {}
    }
  },
  typeormDir: {
    $filter: 'env',
    test: {
      dir: '.'
    },
    $default: {
      dir: './build'
    }
  },
  environment: {
    $filter: 'env',
    test: {
      name: 'local',
      prefix: 't'
    },
    $default: {
      name: { $env: 'ENVIRONMENT_NAME', $default: 'local' },
      prefix: { $env: 'ENVIRONMENT_PREFIX', $default: 'g' }
    }
  },
  new_relic: {
    APP_NAME: { $env: 'APP_NAME' },
    KEY: { $env: 'NEW_RELIC_KEY' },
    enabled: {
      $filter: 'env',
      test: 'false',
      $default: { $env: 'ENABLE_NEW_RELIC', $default: 'true' }
    }
  },

  // Joi validation options
  validationOptions: {
    abortEarly: false, // abort after the last validation error
    stripUnknown: true // remove unknown keys from the validated data
  }
};

internals.store = new Confidence.Store(internals.config);

// eslint-disable-next-line import/prefer-default-export
export const get = function (key: string) {
  return internals.store.get(key, internals.criteria);
};
