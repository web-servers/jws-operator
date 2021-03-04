#!/bin/bash

sed -i -e "s|  annotations:|"\
"  annotations:\n"\
"    categories: Application Runtime\n"\
"    containerImage: ${CONTAINER_IMAGE}\n"\
"    createdAt: ${DATETIME}\n"\
"    support: Red Hat\n"\
"    repository: https://github.com/web-servers/jws-operator\n"\
"    certified: \"false\"\n"\
"    description:\n"\
"      Operator that creates and manages Java applications running on JWS or Tomcat|" \
-e 's|  displayName:.*$|'\
'  displayName: "JBoss Web Server Operator"\n'\
'  description: JWS is based on the ASF Tomcat project.|' \
-e 's|  - base64data: ""|'\
'  - base64data: "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxMDAgMTAwIj48ZGVmcz48c3R5bGU+LmNscy0xe2ZpbGw6I2Q3MWUwMH0uY2xzLTJ7ZmlsbDojYzIxYTAwfS5jbHMtM3tmaWxsOiNmZmZ9LmNscy00e2ZpbGw6I2NkY2RjZH0uY2xzLTV7ZmlsbDojZWFlYWVhfTwvc3R5bGU+PC9kZWZzPjx0aXRsZT5Mb2dvPC90aXRsZT48ZyBpZD0iTGF5ZXJfMSIgZGF0YS1uYW1lPSJMYXllciAxIj48Y2lyY2xlIGNsYXNzPSJjbHMtMSIgY3g9IjUwIiBjeT0iNTAiIHI9IjUwIiB0cmFuc2Zvcm09InJvdGF0ZSgtNDUgNTAgNTApIi8+PHBhdGggY2xhc3M9ImNscy0yIiBkPSJNODUuMzYgMTQuNjRhNTAgNTAgMCAwIDEtNzAuNzIgNzAuNzJ6Ii8+PHBhdGggY2xhc3M9ImNscy0zIiBkPSJNNzEuNjEgMzUuODVMODEgMjYuNDNhMi43NCAyLjc0IDAgMCAwLTIuMjUtLjU4bC0xNC4wOSAyLjY3Yy0uNjQuMTItMS40NS4yMy0yLjM1LjMyTDUwIDQxLjE0IDM3LjcxIDI4Ljg1Yy4zNiAwLTEuNC0uMTUtMi4zNy0uMzRsLTE0LjEyLTIuNjVhMi43NCAyLjc0IDAgMCAwLTIuMjUuNThsOS40MiA5LjQyYy0zIDMuNzItNi42NCA5LjgtNy4wNiAxNy4zN0wyNiA1MS43MWExOS4yNCAxOS4yNCAwIDAgMCAuNzcgMTNsMy40NS0zLjMyUzMwLjI3IDY2IDM2IDcybC44Ni0zIDYuNDEgNi40MWE5LjUyIDkuNTIgMCAwIDAgMTMuNDYgMEw2My4xNCA2OWwuODYgM2M1Ljc2LTYgNS44NS0xMC42MSA1Ljg1LTEwLjYxbDMuNDUgMy4zMmExOS4yNCAxOS4yNCAwIDAgMCAuNzctMTNsNC42NCAxLjUyYy0uNDctNy41OC00LjE1LTEzLjY2LTcuMS0xNy4zOHptLTMzIDIxLjg4QTIuOTEgMi45MSAwIDAgMSAzNiA1My41N2wtLjg4LS44OGExIDEgMCAwIDEgMS40MS0xLjQxbC44OC44OGEyLjkyIDIuOTIgMCAxIDEgMS4yMyA1LjU3ek01NiA2MS42N2wtNSA1djQuNzJhMSAxIDAgMCAxLTIgMHYtNC43NGwtNS01YTEgMSAwIDAgMSAxLjQxLTEuNDFMNTAgNjQuODJsNC41Ny00LjU3QTEgMSAwIDAgMSA1NiA2MS42N3ptOS4xNC05bC0uODguODhhMi45NSAyLjk1IDAgMSAxLTEuNDEtMS40MWwuODgtLjg4YTEgMSAwIDAgMSAxLjQxIDEuNDF6bS40OC0xNS4wN0w1MCA1My4yMmwtMTUuNjEtMTUuNmEuNC40IDAgMCAxIC40Ny0uNjVMNTAgNDQuNTQgNjUuMTQgMzdhLjQuNCAwIDAgMSAuNDcuNjJ6Ii8+PHBhdGggY2xhc3M9ImNscy00IiBkPSJNODEuNDcgMjYuODh6TTY1LjE0IDM3TDUwIDQ0LjU0IDM0Ljg2IDM3YS40LjQgMCAwIDAtLjQ3LjY1TDUwIDUzLjIybDE1LjYxLTE1LjZhLjQuNCAwIDAgMC0uNDctLjYyeiIvPjxwYXRoIGNsYXNzPSJjbHMtNCIgZD0iTTYyLjMxIDI4LjgzYy0yLjg0LjI5LTYuNTYuNDQtOC42Mi40NGgtOC4zNGMtMiAwLTUuMS0uMTQtNy42NC0uNDJMNTAgNDEuMTR6Ii8+PHBhdGggZD0iTTU0LjU3IDYwLjI1TDUwIDY0LjgybC00LjU3LTQuNTdBMSAxIDAgMCAwIDQ0IDYxLjY3bDUgNXY0LjcyYTEgMSAwIDAgMCAyIDB2LTQuNzRsNS01YTEgMSAwIDAgMC0xLjQxLTEuNDF6TTM4LjYzIDUxLjg4YTIuOSAyLjkgMCAwIDAtMS4yMy4yOGwtLjg4LS44OGExIDEgMCAwIDAtMS40MSAxLjQxbC44OC44OGEyLjkyIDIuOTIgMCAxIDAgMi42NS0xLjY5ek02NS4xMyA1MS4yOGExIDEgMCAwIDAtMS40MSAwbC0uODguODhhMi45NSAyLjk1IDAgMSAwIDEuNDEgMS40MWwuODgtLjg4YTEgMSAwIDAgMCAwLTEuNDF6Ii8+PHBhdGggY2xhc3M9ImNscy01IiBkPSJNODIgMjguNjhhMi43NSAyLjc1IDAgMCAwIDAtLjU4di0uMThhMi43IDIuNyAwIDAgMC0uMTMtLjM5bC0uMDctLjE2YTIuNzQgMi43NCAwIDAgMC0uMjktLjQ3IDIuNzkgMi43OSAwIDAgMC0uNDQtLjQ1bC05LjQyIDkuNDJhMzYuMTYgMzYuMTYgMCAwIDEgMy44MiA1LjljNS4yNS0zLjEgNi40My05LjIxIDYuNjEtMTMuMDZ6TTE4IDI4LjY5Yy4xOCAzLjg1IDEuMzYgMTAgNi42MSAxMy4wNmEzNi4xNSAzNi4xNSAwIDAgMSAzLjgyLTUuOUwxOSAyNi40M2EyLjc0IDIuNzQgMCAwIDAtMSAyLjI2eiIvPjwvZz48L3N2Zz4="|' \
-e 's|    mediatype: ""|'\
'    mediatype: "image/svg+xml"|' \
-e 's|  provider: {}|'\
'  provider:\n'\
'    name: Red Hat\n'\
'  links:\n'\
'  - name: JBoss Web Server\n'\
'    url: https://www.redhat.com/en/technologies/jboss-middleware/web-server|' \
-e "s|image: ${IMAGE}|"\
"image: ${CONTAINER_IMAGE}|" \
manifests/jws/$OPERATOR_VERSION/jws-operator.clusterserviceversion.yaml

#Keep scripts that match multiple lines separate from the rest, to avoid matching multiple lines when it's not supposed to
sed -i -z -e 's|  keywords:\n'\
'  - ""|'\
'  keywords:\n'\
'  - "jws"\n'\
'  - "java"\n'\
'  - "open source"\n'\
'  - "application runtime"\n'\
'  - "web server"\n'\
'  - "jboss"|' \
-z -e 's|  maintainers:\n'\
'  - {}|'\
'  maintainers:\n'\
'  - email: support@redhat.com\n'\
'    name: Red Hat|' \
manifests/jws/$OPERATOR_VERSION/jws-operator.clusterserviceversion.yaml