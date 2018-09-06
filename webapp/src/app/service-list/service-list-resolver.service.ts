import { Component, OnInit } from '@angular/core';
import {HttpClient, HttpHeaders } from '@angular/common/http';

import { AuthService } from '../services/auth.service';

import { Injectable }             from '@angular/core';
import { Observable }             from 'rxjs';
import { Router, Resolve, RouterStateSnapshot,
         ActivatedRouteSnapshot } from '@angular/router';


import { ServiceList, ServiceListService }  from './service-list.service';


@Injectable()
export class ServiceListResolver implements Resolve<ServiceList> {

  constructor(private ds: ServiceListService, private router: Router) {}

  resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<ServiceList> {
    return this.ds.getServiceList()
  }
  
}
