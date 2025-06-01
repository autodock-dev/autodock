# autodock
Deploy your Docker Compose stack to AWS without writing any infrastructure code

## Usage
Suppose that you have a docker-compose.yml file in the current directory, you can deploy your code to AWS by running:

```bash
autodock -f docker-compose.yml deploy
```

## Features
- Deploy Docker Compose stack to AWS without having to write any cloudformation, terraform, cdk, or any other infrastructure code.

## How it works
1. autodock will parse the docker-compose.yml file and generate a CloudFormation template for each service, and a "bootstrap" template containing common infrastructure for the entire stack.
2. autodock will deploy the "bootstrap" template first, then deploy each service template.

## Roadmap
- [ ] AWS
    - [x] Deploy application containers as Fargate services.
    - [ ] Deploy Postgres container as a RDS instance.
    - [ ] Deploy Redis container as a ElastiCache instance.
    - [ ] Deploy LocalStack services to real AWS services.
- [ ] Azure support
- [ ] GCP support

## License
The source code is available under the AGPL license. The pre-built autodock binary is free for any purpose including commercial.

## Pricing
To financially support autodock development we may develop a commercial server component. However the pre-built autodock binary is always free for any purpose, and the source code is open source under AGPL license.

## Supporting autodock
If you find autodock useful, please consider supporting it by giving it a star on GitHub.

## Contributing
Issues are welcome!

As we might charge commercial usages for the pre-built binaries, we may not accept contributions to the source code. However, you're freely to use the source code or build your own binary and use it for any purpose allowed by AGPL license.
