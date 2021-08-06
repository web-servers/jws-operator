## webImage (customized images) (Method 3)
Description of the image we use or build.

### applicationImage
The URL of the base image you want to use with the operator. For example:
```
applicationImage: quay.io/jfclere/tomcat10
```
### webApp
How to build the webapp you are adding to the base image.

#### sourceRepositoryURL
The URL where the sources are located. The source should contain a maven pom.xml to allow for a maven build. The produced war is put
in the webapps directory of image.

```
 sourceRepositoryUrl: 'https://github.com/jfclere/demo-webapp.git'
```

#### builder
Tools to build the image.

##### image
The URL of the image you want to use to build the webapp. For example:

```
applicationImage: quay.io/jfclere/tomcat10-builder
```

#### applicationBuildScript:
The script to replace the default script in builder image.
```
applicationBuildScript:
cd tmp; \
etc...
```
