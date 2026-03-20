FROM nginx:alpine

# Copy custom nginx config for SPA routing
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy static site files
COPY . /usr/share/nginx/html/

# Remove the Containerfile and shell scripts from the web root
RUN rm -f /usr/share/nginx/html/Containerfile \
          /usr/share/nginx/html/setup.sh \
          /usr/share/nginx/html/deploy.sh \
          /usr/share/nginx/html/nginx.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
